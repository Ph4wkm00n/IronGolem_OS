#!/usr/bin/env bun
/**
 * png-diff — self-contained PNG pixel-diff with zero npm dependencies.
 *
 * Decodes two PNG files via `node:zlib` (RFC 7950 / W3C PNG spec subset:
 * IHDR + IDAT chunks, filter modes 0-4, 8-bit truecolor or truecolor+alpha)
 * and counts pixels whose RGBA values differ by more than `--per-channel`
 * across all four channels.
 *
 * Exits 0 if `differingPixels / totalPixels < --threshold` (default 0.05);
 * exits 1 otherwise. Prints a single-line JSON summary on stdout either way.
 *
 * Why not pixelmatch+pngjs: this environment can't reach npmjs.org for TLS
 * reasons. Sticking to Node built-ins keeps the visual-regression gate
 * working without an external dependency.
 *
 * Usage:
 *   bun scripts/png-diff.ts <baseline.png> <candidate.png>
 *     [--threshold 0.05] [--per-channel 8] [--quiet]
 */
import { readFileSync } from "node:fs";
import { inflateSync } from "node:zlib";

interface Options {
  readonly baseline: string;
  readonly candidate: string;
  readonly threshold: number;
  readonly perChannel: number;
  readonly quiet: boolean;
}

interface DecodedImage {
  readonly width: number;
  readonly height: number;
  readonly rgba: Uint8Array;
}

const PNG_MAGIC = Uint8Array.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

function parseArgs(argv: readonly string[]): Options {
  const positional: string[] = [];
  let threshold = 0.05;
  let perChannel = 8;
  let quiet = false;

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i]!;
    switch (arg) {
      case "--threshold":
        threshold = Number(argv[++i]);
        break;
      case "--per-channel":
        perChannel = Number(argv[++i]);
        break;
      case "--quiet":
        quiet = true;
        break;
      default:
        positional.push(arg);
    }
  }

  if (positional.length !== 2) {
    console.error("usage: bun scripts/png-diff.ts <baseline.png> <candidate.png>");
    process.exit(2);
  }
  return {
    baseline: positional[0]!,
    candidate: positional[1]!,
    threshold,
    perChannel,
    quiet,
  };
}

function decodePng(filePath: string): DecodedImage {
  const buf = readFileSync(filePath);
  for (let i = 0; i < 8; i++) {
    if (buf[i] !== PNG_MAGIC[i]) {
      throw new Error(`${filePath} is not a PNG (bad magic)`);
    }
  }

  let cursor = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  const idatParts: Buffer[] = [];

  while (cursor < buf.length) {
    const length = buf.readUInt32BE(cursor);
    cursor += 4;
    const type = buf.toString("ascii", cursor, cursor + 4);
    cursor += 4;
    const data = buf.subarray(cursor, cursor + length);
    cursor += length;
    cursor += 4; // skip CRC

    if (type === "IHDR") {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      bitDepth = data.readUInt8(8);
      colorType = data.readUInt8(9);
      const interlace = data.readUInt8(12);
      if (bitDepth !== 8) throw new Error(`${filePath}: only 8-bit depth supported (got ${bitDepth})`);
      if (colorType !== 2 && colorType !== 6) {
        throw new Error(`${filePath}: only RGB(2) or RGBA(6) color types supported (got ${colorType})`);
      }
      if (interlace !== 0) throw new Error(`${filePath}: interlaced PNGs not supported`);
    } else if (type === "IDAT") {
      idatParts.push(data);
    } else if (type === "IEND") {
      break;
    }
  }

  const inflated = inflateSync(Buffer.concat(idatParts));
  const channels = colorType === 6 ? 4 : 3;
  const stride = width * channels;
  const out = new Uint8Array(width * height * 4);

  // Per-line PNG filtering. `prevRow` is the unfiltered scanline above.
  let prevRow = new Uint8Array(stride);
  let srcOffset = 0;
  for (let y = 0; y < height; y++) {
    const filter = inflated[srcOffset++]!;
    const row = new Uint8Array(stride);
    for (let x = 0; x < stride; x++) {
      const raw = inflated[srcOffset + x]!;
      const left = x >= channels ? row[x - channels]! : 0;
      const up = prevRow[x]!;
      const upLeft = x >= channels ? prevRow[x - channels]! : 0;
      let value: number;
      switch (filter) {
        case 0:
          value = raw;
          break;
        case 1:
          value = (raw + left) & 0xff;
          break;
        case 2:
          value = (raw + up) & 0xff;
          break;
        case 3:
          value = (raw + ((left + up) >> 1)) & 0xff;
          break;
        case 4: {
          // Paeth
          const p = left + up - upLeft;
          const pa = Math.abs(p - left);
          const pb = Math.abs(p - up);
          const pc = Math.abs(p - upLeft);
          const paeth = pa <= pb && pa <= pc ? left : pb <= pc ? up : upLeft;
          value = (raw + paeth) & 0xff;
          break;
        }
        default:
          throw new Error(`unknown PNG filter ${filter} on row ${y}`);
      }
      row[x] = value;
    }
    srcOffset += stride;

    // Expand to RGBA.
    const dstBase = y * width * 4;
    for (let x = 0; x < width; x++) {
      const srcPx = x * channels;
      const dstPx = dstBase + x * 4;
      out[dstPx] = row[srcPx]!;
      out[dstPx + 1] = row[srcPx + 1]!;
      out[dstPx + 2] = row[srcPx + 2]!;
      out[dstPx + 3] = channels === 4 ? row[srcPx + 3]! : 0xff;
    }
    prevRow = row;
  }

  return { width, height, rgba: out };
}

function main(): void {
  const opts = parseArgs(process.argv.slice(2));
  const baseline = decodePng(opts.baseline);
  const candidate = decodePng(opts.candidate);

  if (baseline.width !== candidate.width || baseline.height !== candidate.height) {
    const summary = {
      ok: false,
      reason: "size-mismatch",
      baseline: { width: baseline.width, height: baseline.height },
      candidate: { width: candidate.width, height: candidate.height },
    };
    process.stdout.write(JSON.stringify(summary) + "\n");
    process.exit(1);
  }

  const total = baseline.width * baseline.height;
  let diffPixels = 0;
  for (let i = 0; i < total; i++) {
    const off = i * 4;
    const dr = Math.abs(baseline.rgba[off]! - candidate.rgba[off]!);
    const dg = Math.abs(baseline.rgba[off + 1]! - candidate.rgba[off + 1]!);
    const db = Math.abs(baseline.rgba[off + 2]! - candidate.rgba[off + 2]!);
    const da = Math.abs(baseline.rgba[off + 3]! - candidate.rgba[off + 3]!);
    if (dr > opts.perChannel || dg > opts.perChannel || db > opts.perChannel || da > opts.perChannel) {
      diffPixels++;
    }
  }

  const fraction = diffPixels / total;
  const ok = fraction < opts.threshold;
  const summary = {
    ok,
    diffPixels,
    totalPixels: total,
    fraction: Number(fraction.toFixed(6)),
    threshold: opts.threshold,
    perChannel: opts.perChannel,
  };
  if (!opts.quiet) process.stdout.write(JSON.stringify(summary) + "\n");
  process.exit(ok ? 0 : 1);
}

main();
