{{/*
Expand the name of the chart.
*/}}
{{- define "lumo-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "lumo-agent.fullname" -}}
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
{{- define "lumo-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "lumo-agent.labels" -}}
helm.sh/chart: {{ include "lumo-agent.chart" . }}
{{ include "lumo-agent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels for node monitor (DaemonSet)
*/}}
{{- define "lumo-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "lumo-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: lumo
{{- end }}

{{/*
Selector labels for node monitor
*/}}
{{- define "lumo-agent.nodeSelectorLabels" -}}
{{ include "lumo-agent.selectorLabels" . }}
app.kubernetes.io/component: node-monitor
{{- end }}

{{/*
Selector labels for cluster monitor
*/}}
{{- define "lumo-agent.clusterSelectorLabels" -}}
{{ include "lumo-agent.selectorLabels" . }}
app.kubernetes.io/component: cluster-monitor
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "lumo-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "lumo-agent.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the image name
*/}}
{{- define "lumo-agent.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Create the namespace
*/}}
{{- define "lumo-agent.namespace" -}}
{{- default .Release.Namespace .Values.global.namespace }}
{{- end }}
