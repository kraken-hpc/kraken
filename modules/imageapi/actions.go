/* actions.go: methods for handling imageapi actions
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import ia "github.com/kraken-hpc/kraken/extensions/imageapi"

//////////////////////
// Action handling //
////////////////////

func (is *ImageAPI) setRunning() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.running = true
}

func (is *ImageAPI) isRunning() bool {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	return is.running
}

func (is *ImageAPI) clearRunning() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.running = false
}

func (is *ImageAPI) setAction(name string, action ia.Image_Action) {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.actions[name] = action
}

// note: also a trigger
func (is *ImageAPI) clearAction(name string) {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	if _, ok := is.actions[name]; ok {
		delete(is.actions, name)
	}
}

func (is *ImageAPI) clearAllActions() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.actions = make(map[string]ia.Image_Action)
}

func (is *ImageAPI) getAction(name string) ia.Image_Action {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	if a, ok := is.actions[name]; ok {
		return a
	}
	return ia.Image_NONE
}
