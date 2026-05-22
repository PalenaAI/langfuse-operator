{{/*
Expand the name of the chart.
*/}}
{{- define "langfuse-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "langfuse-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "langfuse-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "langfuse-operator.labels" -}}
helm.sh/chart: {{ include "langfuse-operator.chart" . }}
{{ include "langfuse-operator.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default (printf "v%s" .Chart.AppVersion) | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "langfuse-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "langfuse-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "langfuse-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "langfuse-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Manager container image.
Default tag is `v<Chart.appVersion>` to match the release workflow, which
publishes images as `vX.Y.Z`. If the user sets `image.tag` explicitly it is
used verbatim (so `--set image.tag=v0.6.3` and `--set image.tag=latest` both
work).
*/}}
{{- define "langfuse-operator.image" -}}
{{- $tag := .Values.image.tag | default (printf "v%s" .Chart.AppVersion) -}}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
