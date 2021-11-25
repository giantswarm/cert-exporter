{{- define "certExporter.containerImage" -}}
{{- if .Values.image.tag }}
{{- .Values.registry.domain }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}
{{- else }}
{{- .Values.registry.domain }}/{{ .Values.image.repository }}:{{ .Chart.AppVersion }}
{{- end -}}
{{- end -}}

{{/* Create a default fully qualified app name. Truncated to meet DNS naming spec. */}}
{{- define "certExporter.name" -}}
{{- default .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "certExporter.daemonset.name" -}}
{{- printf "%s-%s" ( include "certExporter.name" . | trunc 54 | trimSuffix "-" ) "daemonset" -}}
{{- end -}}

{{- define "certExporter.deployment.name" -}}
{{- printf "%s-%s" ( include "certExporter.name" . | trunc 53 | trimSuffix "-" ) "deployment" -}}
{{- end -}}

{{/* Create chart name and version as used by the chart label. */}}
{{- define "certExporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "certExporter.commonLabels" -}}
app.kubernetes.io/managed-by: "{{ .Release.Service }}"
app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
helm.sh/chart: "{{ template "certExporter.chart" . }}"
{{- end -}}

{{- define "certExporter.daemonset.matchLabels" -}}
app.kubernetes.io/name: "{{ template "certExporter.daemonset.name" . }}"
app.kubernetes.io/instance: "{{ template "certExporter.name" . }}"
{{- end -}}

{{- define "certExporter.deployment.matchLabels" -}}
app.kubernetes.io/name: "{{ template "certExporter.deployment.name" . }}"
app.kubernetes.io/instance: "{{ template "certExporter.name" . }}"
{{- end -}}

{{- define "certExporter.daemonset.labels" -}}
{{ include "certExporter.commonLabels" . }}
{{ include "certExporter.daemonset.matchLabels" . }}
{{- end -}}

{{- define "certExporter.deployment.labels" -}}
{{ include "certExporter.commonLabels" . }}
{{ include "certExporter.deployment.matchLabels" . }}
{{- end -}}

{{- define "certExporter.fixJob" -}}
{{- printf "%s-%s" ( include "certExporter.name" . | trunc 56 | trimSuffix "-" ) "fix-job" -}}
{{- end -}}

{{- define "certExporter.fixJobSelectorLabels" -}}
app.kubernetes.io/name: "{{ template "certExporter.fixJob" . }}"
app.kubernetes.io/instance: "{{ template "certExporter.fixJob" . }}"
{{- end -}}

{{- define "certExporter.fixJobAnnotations" -}}
"helm.sh/hook": "pre-install,pre-upgrade"
"helm.sh/hook-delete-policy": "before-hook-creation,hook-succeeded,hook-failed"
{{- end -}}

{{/* Create a label which can be used to select any orphaned fix-job hook resources */}}
{{- define "certExporter.fixJobSelector" -}}
{{- printf "%s" "fix-job-hook" -}}
{{- end -}}
