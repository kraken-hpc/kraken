// Code generated by kraken {{ .Kraken.Version }}. DO NOT EDIT
package main

// Extensions
{{ range .Extensions }}
import _ "{{ . }}"
{{ end }}

// Modules
{{ range .Modules }}
import _ "{{ . }}"
{{ end }}