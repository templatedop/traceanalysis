{{/*
Expand the name of the chart.
*/}}
{{- define "observability-analysis.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "observability-analysis.fullname" -}}
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
{{- define "observability-analysis.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "observability-analysis.labels" -}}
helm.sh/chart: {{ include "observability-analysis.chart" . }}
{{ include "observability-analysis.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "observability-analysis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "observability-analysis.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Server selector labels
*/}}
{{- define "observability-analysis.serverSelectorLabels" -}}
{{ include "observability-analysis.selectorLabels" . }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Worker selector labels
*/}}
{{- define "observability-analysis.workerSelectorLabels" -}}
{{ include "observability-analysis.selectorLabels" . }}
app.kubernetes.io/component: worker
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "observability-analysis.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "observability-analysis.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the proper image name
*/}}
{{- define "observability-analysis.image" -}}
{{- $registryName := .Values.global.imageRegistry -}}
{{- $repositoryName := .Values.server.image.repository -}}
{{- $tag := .Values.server.image.tag | default .Chart.AppVersion -}}
{{- if $registryName }}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- else }}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
Return the proper worker image name
*/}}
{{- define "observability-analysis.workerImage" -}}
{{- $registryName := .Values.global.imageRegistry -}}
{{- $repositoryName := .Values.worker.image.repository -}}
{{- $tag := .Values.worker.image.tag | default .Chart.AppVersion -}}
{{- if $registryName }}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- else }}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
Return the namespace
*/}}
{{- define "observability-analysis.namespace" -}}
{{- default .Release.Namespace .Values.namespace }}
{{- end }}

{{/*
Return the secret name
*/}}
{{- define "observability-analysis.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "observability-analysis.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Return the configmap name
*/}}
{{- define "observability-analysis.configMapName" -}}
{{- include "observability-analysis.fullname" . }}-config
{{- end }}

{{/*
Return the PVC name
*/}}
{{- define "observability-analysis.pvcName" -}}
{{- if .Values.persistence.existingClaim }}
{{- .Values.persistence.existingClaim }}
{{- else }}
{{- include "observability-analysis.fullname" . }}-knowledge
{{- end }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "observability-analysis.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}
