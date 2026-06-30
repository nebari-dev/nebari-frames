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
