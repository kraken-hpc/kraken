/* includes.go.tpl: used to include modules & extensions specified in build config
 * 
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package main

// Extensions
{{ range .Extensions }}
import _ "{{ . }}"
{{ end }}

// Modules
{{ range .Modules }}
import _ "{{ . }}"
{{ end }}
