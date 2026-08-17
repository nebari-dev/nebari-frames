{{- define "nebari-frames.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nebari-frames.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "nebari-frames.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "nebari-frames.labels" -}}
app.kubernetes.io/name: {{ include "nebari-frames.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{- define "nebari-frames.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nebari-frames.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "nebari-frames.oidcSecretName" -}}
{{- printf "%s-oidc-client" (include "nebari-frames.fullname" .) -}}
{{- end -}}

{{/*
"true" when any branding value is set, empty otherwise. Gates the runtime-config
ConfigMap, its volume mount, and BRANDING_CONFIG_FILE, so an unbranded install
renders exactly the manifests it did before branding existed.
*/}}
{{- define "nebari-frames.hasBranding" -}}
{{- $b := .Values.branding -}}
{{- if or $b.title $b.logoUrl $b.logoUrlDark $b.faviconUrl $b.theme.light $b.theme.dark -}}
true
{{- end -}}
{{- end -}}

{{/*
The runtime configuration document the app serves at /config.json, as JSON. It
carries the branding fields today; the generic name leaves room for other
runtime settings the SPA may need later. Only non-empty values are emitted so
the app (and the SPA) falls back to its built-in default per field; an all-empty
branding block yields "{}".
*/}}
{{- define "nebari-frames.configJson" -}}
{{- $doc := dict -}}
{{- with .Values.branding.title }}{{- $doc = set $doc "title" . -}}{{- end -}}
{{- with .Values.branding.logoUrl }}{{- $doc = set $doc "logoUrl" . -}}{{- end -}}
{{- with .Values.branding.logoUrlDark }}{{- $doc = set $doc "logoUrlDark" . -}}{{- end -}}
{{- with .Values.branding.faviconUrl }}{{- $doc = set $doc "faviconUrl" . -}}{{- end -}}
{{- $theme := dict -}}
{{- with .Values.branding.theme.light }}{{- $theme = set $theme "light" . -}}{{- end -}}
{{- with .Values.branding.theme.dark }}{{- $theme = set $theme "dark" . -}}{{- end -}}
{{- if $theme }}{{- $doc = set $doc "theme" $theme -}}{{- end -}}
{{- $doc | toPrettyJson -}}
{{- end -}}

{{- define "nebari-frames.configMapName" -}}
{{- printf "%s-config" (include "nebari-frames.fullname" .) -}}
{{- end -}}

{{/* Directory the config ConfigMap is mounted at, and the file inside it. */}}
{{- define "nebari-frames.configMountPath" -}}
/etc/nebari-frames/config
{{- end -}}

{{- define "nebari-frames.configFilePath" -}}
{{- printf "%s/config.json" (include "nebari-frames.configMountPath" .) -}}
{{- end -}}

{{/*
Public URL for the MCP endpoint: explicit mcp.publicUrl, else derived from the
NebariApp hostname. Empty string when neither is available (endpoint stays off).
*/}}
{{- define "nebari-frames.mcpPublicUrl" -}}
{{- if .Values.mcp.publicUrl -}}
{{- .Values.mcp.publicUrl -}}
{{- else if .Values.nebariapp.enabled -}}
{{- printf "https://%s" (required "nebariapp.hostname is required when mcp.enabled and mcp.publicUrl is unset" .Values.nebariapp.hostname) -}}
{{- end -}}
{{- end -}}
