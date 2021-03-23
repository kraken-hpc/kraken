/* mutations.go: mutation handlers for imageapi
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	math "math"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hpc/kraken/core"
	ia "github.com/hpc/kraken/extensions/imageapi"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

////////////////////////
// Mutation handlers //
//////////////////////

func (is *ImageAPI) mUKtoIDLE(me *core.MutationEvent) {
	// force a discovery run and go to idle
	is.clearAllTriggers()
	is.mutex.Lock()
	is.target = ia.ImageState_IDLE
	is.mutex.Unlock()

	is.discover()
}

func (is *ImageAPI) mANYtoACTIVE(me *core.MutationEvent) {
	// Activate all Images (if they aren't already)
	is.mutex.Lock()
	is.target = ia.ImageState_ACTIVE
	is.mutex.Unlock()
	is.setRunning()
	v, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	isd := v.(ia.ImageSet)
	for name, _ := range isd.Images {
		is.activateImage(name)
	}
}

func (is *ImageAPI) activateImage(name string) {
	if !is.isRunning() || is.target != ia.ImageState_ACTIVE {
		// do nothing
		return
	}
	var cfgImage, dscImage ia.Image

	imgURL := util.URLPush(util.URLPush(isURL, "Images"), name)
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), imgURL)
	if err != nil {
		// shouldn't have been called
		is.api.Logf(types.LLDDEBUG, "actiaveImage called on an image that has no dsc state: %s", name)
	}
	dscImage = v.(ia.Image)

	action := dscImage.Action
	if action == is.getAction(name) {
		// we're already doing it...
		is.api.Logf(types.LLDDDEBUG, "asked to perform an action we're already performing: %s on %s", action.String(), name)
		return
	}
	is.api.Logf(types.LLDDEBUG, "activating %s (%s), dsc: %v\n", name, action.String(), spew.Sdump(dscImage))

	if action == ia.Image_DELETE {
		is.setAction(name, ia.Image_DELETE)
		is.deleteImage(name, &ia.Image{})
		is.setTrigger(name, ContainerState_DELETED, is.clearAction)
		return
	}

	// from this point we need the cfgImage
	v, err = is.api.QueryGetValue(is.api.Self().String(), imgURL)
	if err != nil {
		is.api.Logf(types.LLERROR, "asked to perform a non-delete action on a non-existent image: %s : %s", name, action.String())
		return
	}
	cfgImage = v.(ia.Image)

	switch action {
	case ia.Image_CREATE:
		is.setAction(name, ia.Image_CREATE)
		is.createImage(name, &cfgImage)
	case ia.Image_RELOAD:
		is.setAction(name, ia.Image_RELOAD)
		is.reloadImage(name, &cfgImage)
	case ia.Image_NONE:
		return
	}
	is.setTrigger(name, cfgImage.Container.State, is.tSetACTIVE)
	is.setTrigger(name, cfgImage.Container.State, is.clearAction)
}

func (is *ImageAPI) mANYtoIDLE(me *core.MutationEvent) {
	// Stop & delete all Images
	is.setRunning()
	is.mutex.Lock()
	is.target = ia.ImageState_IDLE
	is.mutex.Unlock()
	v, _ := me.NodeCfg.GetValue(isURL)
	isc := v.Interface().(ia.ImageSet)
	v, _ = me.NodeDsc.GetValue(isURL)
	isd := v.Interface().(ia.ImageSet)
	for name, image := range isc.Images {
		if _, ok := isd.Images[name]; ok {
			// this container needs to be deleted
			is.setAction(name, ia.Image_DELETE)
			is.deleteImage(name, image)
			is.setTrigger(name, ContainerState_DELETED, is.tSetIDLE)
			is.setTrigger(name, ContainerState_DELETED, is.clearAction)
		}
	}
}

func (is *ImageAPI) mRecoverError(me *core.MutationEvent) bool {
	// logic for when an error is recoverable
	cleared := true
	changed := false
	is.mutex.Lock()
	v, _ := is.api.QueryGetValueDsc(is.api.Self().String(), isURL)
	isd := v.(ia.ImageSet)
	for name, image := range isd.Images {
		if image.State == ia.ImageState_ERROR {
			image.Retries++
			changed = true
			if image.Retries > is.cfg.MaxRetries {
				// too many retries
				cleared = false
				image.State = ia.ImageState_FATAL
				image.LastError = ia.Image_MAX_ATTEMPTS
				is.api.Logf(types.LLERROR, "image reached maximum retries (%d/%d): %s", image.Retries, is.cfg.MaxRetries, name)
			} else {
				image.State = ia.ImageState_UPDATE
				image.Action = ia.Image_RELOAD
				backoff := int(5 * math.Pow(2, float64(image.Retries-1))) // FIXME: this probably should be configurable
				is.api.Logf(types.LLERROR, "image is retrying (%d/%d) in %ds: %s", image.Retries, is.cfg.MaxRetries, backoff, name)
				time.Sleep(time.Duration(backoff) * time.Second)
			}
		}
	}
	if changed {
		if err := is.api.QuerySetValueDsc(is.api.Self().String(), isURL, isd); err != nil {
			is.api.Logf(types.LLERROR, "failed to set updated error value(s): %v", err)
		}
	}
	is.mutex.Unlock()
	if changed {
		is.updateSetState()
	}
	return cleared
}

func (is *ImageAPI) setValues(vs map[string]interface{}) {
	is.mutex.Lock()
	if _, err := is.api.QuerySetValuesDsc(is.api.Self().String(), vs); err != nil {
		is.api.Logf(types.LLERROR, "setvalues failed: %v: %v", err, vs)
	}
	is.mutex.Unlock()
}
