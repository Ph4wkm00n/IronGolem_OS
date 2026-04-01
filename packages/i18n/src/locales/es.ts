/**
 * Spanish translations for IronGolem OS.
 */

import type { en } from "./en";

type TranslationKeys = typeof en;

export const es: TranslationKeys = {
  // ---------------------------------------------------------------------------
  // Primary navigation
  // ---------------------------------------------------------------------------
  "nav.home": "Inicio",
  "nav.inbox": "Bandeja de entrada",
  "nav.recipes": "Recetas",
  "nav.squads": "Equipos",
  "nav.health": "Centro de salud",
  "nav.settings": "Configuración",
  "nav.admin": "Consola de administración",
  "nav.plugins": "Complementos",
  "nav.research": "Investigación",
  "nav.history": "Historial",
  "nav.connectors": "Conectores",
  "nav.fleet": "Flota",

  // ---------------------------------------------------------------------------
  // Heartbeat status labels
  // ---------------------------------------------------------------------------
  "heartbeat.healthy": "Saludable",
  "heartbeat.quietly_recovering": "Recuperándose silenciosamente",
  "heartbeat.needs_attention": "Necesita atención",
  "heartbeat.paused": "Pausado",
  "heartbeat.quarantined": "En cuarentena",
  "heartbeat.unknown": "Desconocido",
  "heartbeat.last_seen": "Visto por última vez {{time}}",
  "heartbeat.never_reported": "Nunca reportado",

  // ---------------------------------------------------------------------------
  // Risk level labels
  // ---------------------------------------------------------------------------
  "risk.none": "Sin riesgo",
  "risk.low": "Riesgo bajo",
  "risk.medium": "Riesgo medio",
  "risk.high": "Riesgo alto",
  "risk.critical": "Riesgo crítico",
  "risk.description": "Nivel de riesgo: {{level}}",

  // ---------------------------------------------------------------------------
  // Safety card section headers
  // ---------------------------------------------------------------------------
  "safety.what_it_does": "Qué hace",
  "safety.what_it_needs": "Qué necesita",
  "safety.what_could_go_wrong": "Qué podría salir mal",
  "safety.how_to_undo": "Cómo deshacer",
  "safety.permissions_required": "Permisos requeridos",
  "safety.data_accessed": "Datos accedidos",
  "safety.estimated_impact": "Impacto estimado",
  "safety.rollback_available": "Reversión disponible",

  // ---------------------------------------------------------------------------
  // Approval action labels
  // ---------------------------------------------------------------------------
  "approval.approve": "Aprobar",
  "approval.reject": "Rechazar",
  "approval.approve_once": "Aprobar una vez",
  "approval.approve_always": "Aprobar siempre",
  "approval.review_details": "Revisar detalles",
  "approval.pending": "Pendiente de aprobación",
  "approval.approved": "Aprobado",
  "approval.rejected": "Rechazado",
  "approval.expired": "Expirado",
  "approval.request_from": "Solicitud de {{agent}}",
  "approval.action_description": "Acción: {{action}}",

  // ---------------------------------------------------------------------------
  // Recipe labels
  // ---------------------------------------------------------------------------
  "recipe.activate": "Activar receta",
  "recipe.deactivate": "Desactivar receta",
  "recipe.edit": "Editar receta",
  "recipe.duplicate": "Duplicar receta",
  "recipe.delete": "Eliminar receta",
  "recipe.active": "Activa",
  "recipe.inactive": "Inactiva",
  "recipe.draft": "Borrador",
  "recipe.running": "Ejecutándose",
  "recipe.paused": "Pausada",
  "recipe.created_by": "Creado por {{author}}",
  "recipe.last_run": "Última ejecución {{time}}",

  // ---------------------------------------------------------------------------
  // Squad labels
  // ---------------------------------------------------------------------------
  "squad.inbox": "Equipo de bandeja de entrada",
  "squad.research": "Equipo de investigación",
  "squad.ops": "Equipo de operaciones",
  "squad.security": "Equipo de seguridad",
  "squad.executive_assistant": "Equipo de asistente ejecutivo",
  "squad.agents_count": "{{count}} agentes",
  "squad.active_tasks": "{{count}} tareas activas",

  // ---------------------------------------------------------------------------
  // Common UI phrases
  // ---------------------------------------------------------------------------
  "common.loading": "Cargando...",
  "common.error": "Se produjo un error",
  "common.retry": "Reintentar",
  "common.cancel": "Cancelar",
  "common.save": "Guardar",
  "common.delete": "Eliminar",
  "common.confirm": "Confirmar",
  "common.close": "Cerrar",
  "common.back": "Atrás",
  "common.next": "Siguiente",
  "common.search": "Buscar",
  "common.filter": "Filtrar",
  "common.sort": "Ordenar",
  "common.refresh": "Actualizar",
  "common.no_results": "No se encontraron resultados",
  "common.created_at": "Creado {{time}}",
  "common.updated_at": "Actualizado {{time}}",
  "common.version": "Versión {{version}}",
  "common.enabled": "Habilitado",
  "common.disabled": "Deshabilitado",
  "common.on": "Activado",
  "common.off": "Desactivado",
  "common.yes": "Sí",
  "common.no": "No",
  "common.unknown": "Desconocido",
  "common.all": "Todos",
  "common.none": "Ninguno",
  "common.more": "Más",
  "common.less": "Menos",
  "common.show_all": "Mostrar todo",
  "common.collapse": "Contraer",
  "common.expand": "Expandir",

  // ---------------------------------------------------------------------------
  // Settings
  // ---------------------------------------------------------------------------
  "settings.general": "General",
  "settings.appearance": "Apariencia",
  "settings.language": "Idioma",
  "settings.notifications": "Notificaciones",
  "settings.security": "Seguridad",
  "settings.integrations": "Integraciones",
  "settings.workspace": "Espacio de trabajo",
  "settings.deployment_mode": "Modo de despliegue",
  "settings.solo_mode": "Modo individual",
  "settings.household_mode": "Modo hogar",
  "settings.team_mode": "Modo equipo",

  // ---------------------------------------------------------------------------
  // Plugins
  // ---------------------------------------------------------------------------
  "plugin.install": "Instalar complemento",
  "plugin.uninstall": "Desinstalar complemento",
  "plugin.enable": "Habilitar complemento",
  "plugin.disable": "Deshabilitar complemento",
  "plugin.installed": "Instalado",
  "plugin.available": "Disponible",
  "plugin.update_available": "Actualización disponible",
  "plugin.permissions": "Permisos del complemento",
  "plugin.by_author": "Por {{author}}",
} as const;
