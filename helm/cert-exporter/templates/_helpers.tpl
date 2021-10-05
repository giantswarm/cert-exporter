{{- define "certExporter.containerImage" -}}
{{- if .Values.image.tag }}
{{- .Values.registry.domain }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}
{{- else }}
{{- .Values.registry.domain }}/{{ .Values.image.repository }}:{{ .Chart.AppVersion }}
{{- end -}}
{{- end -}}
