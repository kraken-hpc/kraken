/* operations.go: this file containes imageapi methods that perform operations on images
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2020, Triad National Security, LLC
 * See LICENSE file for details.
 */

package imageapi

import (
	"encoding/json"
	fmt "fmt"

	transport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	ia "github.com/hpc/kraken/extensions/imageapi"
	kjson "github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
	api "github.com/jlowellwofford/imageapi/client"
	"github.com/jlowellwofford/imageapi/client/containers"
	"github.com/jlowellwofford/imageapi/models"
)

////////////////////////
// Utility functions //
//////////////////////

// get a configured API client
func (is ImageAPI) getAPIClient() *api.Imageapi {
	as := is.cfg.ApiServer
	t := transport.New(fmt.Sprintf("%s:%d", as.Server, as.Port), as.ApiBase, []string{"http"})
	return api.New(t, strfmt.Default)
}

// not super efficient, but should work
func (is *ImageAPI) protoToAPI(p *ia.Container) *models.Container {
	b, err := json.Marshal(p)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container proto -> API: %v", err)
		return nil
	}
	r := &models.Container{}
	err = json.Unmarshal(b, r)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container proto -> API: %v", err)
		return nil
	}
	return r
}

// not super efficient, but should work
func (is *ImageAPI) apiToProto(a *models.Container) *ia.Container {
	b, err := json.Marshal(a)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container API -> proto: %v", err)
		return nil
	}
	r := &ia.Container{}
	kjson.Unmarshal(b, r)
	if err != nil {
		is.api.Logf(types.LLFATAL, "error translating container API -> proto: %v", err)
		return nil
	}
	return r
}

func (is *ImageAPI) imageRaiseError(name string, e ia.Image_ErrorCode) {
	url := util.URLPush(util.URLPush(isURL, "Images"), name)
	is.setValues(map[string]interface{}{
		util.URLPush(url, "State"):     ia.ImageState_ERROR,
		util.URLPush(url, "LastError"): e,
	})
}

func (is *ImageAPI) createImage(name string, image *ia.Image) {
	is.api.Logf(types.LLINFO, "creating image %s", name)
	// make sure naming is uniform.  The container must be named the same as the map.
	image.Container.Name = name
	client := is.getAPIClient()
	apiContainer := is.protoToAPI(image.Container)
	params := containers.NewCreateContainerParams()
	params.Container = apiContainer
	_, err := client.Containers.CreateContainer(params)
	if err != nil {
		if cerr, ok := err.(*containers.CreateContainerDefault); ok {
			is.api.Logf(types.LLERROR, "container creation failed for image %s: Code: %d Message: %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			// "discover" our error state
		} else {
			is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
		}
		is.imageRaiseError(name, ia.Image_ATTACH)
	}
}

func (is *ImageAPI) deleteImage(name string, image *ia.Image) {
	url := isURL
	for _, u := range []string{"Images", name, "Container", "State"} {
		url = util.URLPush(url, u)
	}
	v, err := is.api.QueryGetValueDsc(is.api.Self().String(), url)
	if err != nil || v.(string) == ContainerState_DELETED {
		// image must not exist, so our work is done
		is.api.Logf(types.LLDDDEBUG, "refusing to delete an already deleted image: %s", name)
		return
	}
	client := is.getAPIClient()
	switch v.(string) {
	case ContainerState_RUNNING:
		// tell the node to stop
		is.api.Logf(types.LLINFO, "stopping image %s", name)
		params := containers.NewSetContainerStateBynameParams()
		params.Name = name
		params.State = string(models.ContainerStateExited)
		_, err := client.Containers.SetContainerStateByname(params)
		if err != nil {
			if cerr, ok := err.(*containers.SetContainerStateBynameDefault); ok {
				is.api.Logf(types.LLERROR, "failed to stop container %s: Code: %d Message %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			} else {
				is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
			}
			// "discover" our error state
			is.imageRaiseError(name, ia.Image_ATTACH)
		}
		fallthrough
	case ContainerState_STOPPING:
		// set a trigger and get rerun when it's done
		is.api.Logf(types.LLINFO, "waiting for image %s to exit", name)
		is.setTrigger(name, ContainerState_EXITED, is.tDelete)
		return
	}
	// ok, we're ready to delete
	is.api.Logf(types.LLINFO, "deleting image %s", name)
	params := containers.NewDeleteContainerBynameParams()
	params.Name = name
	_, err = client.Containers.DeleteContainerByname(params)
	if err != nil {
		if cerr, ok := err.(*containers.DeleteContainerBynameDefault); ok {
			is.api.Logf(types.LLERROR, "failed to delete container %s: Code: %d Message %s", name, cerr.Payload.Code, *cerr.Payload.Message)
			// "discover" our error state
		} else {
			is.api.Logf(types.LLERROR, "failed to call imageapi: %v", err)
		}
		is.imageRaiseError(name, ia.Image_ATTACH)
	}
}

func (is *ImageAPI) reloadImage(name string, image *ia.Image) {
	is.api.Logf(types.LLINFO, "reloading image %s", name)
	is.setTrigger(name, ContainerState_DELETED, is.tCreate)
	is.deleteImage(name, image)
}
