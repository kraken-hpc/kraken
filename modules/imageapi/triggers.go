/* triggers.go: functions for handling imageapi module triggers
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	ia "github.com/kraken-hpc/kraken/extensions/imageapi"
	"github.com/kraken-hpc/kraken/lib/types"
	"github.com/kraken-hpc/kraken/lib/util"
)

///////////////////////
// trigger handling //
/////////////////////

// Triggers are hooks that get called when a certain ContainerState is reached
// This allows for chaining of async actions
// All triggers take an Image name as argument

// we define an extra container state to recognize deleted containers
const (
	ContainerState_DELETED  = "DELETED"
	ContainerState_CREATED  = "created"
	ContainerState_RUNNING  = "running"
	ContainerState_STOPPING = "stopping"
	ContainerState_EXITED   = "exited"
	ContainerState_DEAD     = "dead"
)

func (is *ImageAPI) clearAllTriggers() {
	is.api.Logf(types.LLDDDEBUG, "clearing all triggers")
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.triggers = make(map[string]map[string][]trigger)
}

func (is *ImageAPI) clearTriggers(name string) {
	is.api.Logf(types.LLDDDEBUG, "clearing triggers for %s", name)
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.triggers[name] = make(map[string][]trigger)
}

func (is *ImageAPI) setTrigger(name, state string, t trigger) {
	is.api.Logf(types.LLDDDEBUG, "setting trigger %s for %s", state, name)
	is.mutex.Lock()
	defer is.mutex.Unlock()
	var ok bool
	var ts map[string][]trigger
	if ts, ok = is.triggers[name]; !ok {
		is.triggers[name] = make(map[string][]trigger)
		ts = is.triggers[name]
	}
	ts[state] = append(ts[state], t)
}

func (is *ImageAPI) fireTriggers(name, state string) bool {
	is.mutex.Lock()
	if ts, ok := is.triggers[name][state]; ok {
		if len(ts) == 0 {
			is.mutex.Unlock()
			return false
		}
		is.api.Logf(types.LLDDEBUG, "firing image %d triggers %s for %s", len(ts), state, name)
		delete(is.triggers[name], state)
		is.mutex.Unlock()
		for _, t := range ts {
			t(name)
		}
		return true
	}
	is.mutex.Unlock()
	return false
}

func (is *ImageAPI) tDelete(name string) {
	// we use an empty image
	is.deleteImage(name, &ia.Image{})
}

func (is *ImageAPI) tCreate(name string) {
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	v, err := is.api.QueryGetValue(is.api.Self().String(), iurl)
	image := v.(ia.Image)
	if err != nil {
		// apparently there's nothing to create
		is.api.Logf(types.LLDEBUG, "triggered to create image that is no longer defined: %s", name)
		return
	}
	is.createImage(name, &image)
}

func (is *ImageAPI) tSetACTIVE(name string) {
	is.api.Logf(types.LLINFO, "image %s becomes ACTIVE", name)
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(iurl, "State"):   ia.ImageState_ACTIVE,
		util.URLPush(iurl, "Action"):  ia.Image_NONE,
		util.URLPush(iurl, "Retries"): int32(0),
	})
}

func (is *ImageAPI) tSetIDLE(name string) {
	is.api.Logf(types.LLINFO, "image %s becomes IDLE", name)
	iurl := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(iurl, "State"):   ia.ImageState_IDLE,
		util.URLPush(iurl, "Action"):  ia.Image_CREATE,
		util.URLPush(iurl, "Retries"): int32(0),
	})
}
