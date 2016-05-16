// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	//	"net/http"

	"runtime"

	"github.com/go-swagger/go-swagger/httpkit/middleware"

	"github.com/vmware/vic/apiservers/docker/models"
	"github.com/vmware/vic/apiservers/docker/restapi/operations"
	"github.com/vmware/vic/apiservers/docker/restapi/operations/misc"
)

type MiscHandlersImpl struct{}

func (handlers *MiscHandlersImpl) Configure(api *operations.DockerAPI) {
	api.MiscCheckAuthenticationHandler = misc.CheckAuthenticationHandlerFunc(handlers.CheckAuthentication)
	api.MiscGetEventsHandler = misc.GetEventsHandlerFunc(handlers.GetEvents)
	api.MiscGetSystemInformationHandler = misc.GetSystemInformationHandlerFunc(handlers.GetSystemInfo)
	api.MiscGetVersionHandler = misc.GetVersionHandlerFunc(handlers.GetVersion)
	api.MiscPingHandler = misc.PingHandlerFunc(handlers.Ping)
}

func (handlers *MiscHandlersImpl) CheckAuthentication(params misc.CheckAuthenticationParams) middleware.Responder {
	return middleware.NotImplemented("operation misc.CheckAuthentication has not yet been implemented")
}

func (handlers *MiscHandlersImpl) GetEvents(params misc.GetEventsParams) middleware.Responder {
	return middleware.NotImplemented("operation misc.GetEvents has not yet been implemented")
}

func (handlers *MiscHandlersImpl) GetSystemInfo() middleware.Responder {
	Driver := "Portlayer Storage"
	IndexServerAddress := "https://index.docker.io/v1/"
	ServerVersion := "0.0.1"
	Name := "VIC"

	info := &models.SystemInformation{
		Driver:             &Driver,
		IndexServerAddress: &IndexServerAddress,
		ServerVersion:      &ServerVersion,
		Name:               &Name,
	}
	return misc.NewGetSystemInformationOK().WithPayload(info)
}

func (handlers *MiscHandlersImpl) GetVersion() middleware.Responder {
	APIVersion := "1.22"
	Arch := runtime.GOARCH
	// FIXME: fill with real build time
	BuildTime := "-"
	Experimental := true
	// FIXME: fill with real commit id
	GitCommit := "-"
	GoVersion := runtime.Version()
	// FIXME: fill with real kernel version
	KernelVersion := "-"
	Os := runtime.GOOS
	Version := "0.0.1"

	// go runtime panics without this so keep this here
	// until we find a repro case and report it to upstream
	_ = Arch

	version := &models.Version{
		APIVersion:    &APIVersion,
		Arch:          &Arch,
		BuildTime:     &BuildTime,
		Experimental:  &Experimental,
		GitCommit:     &GitCommit,
		GoVersion:     &GoVersion,
		KernelVersion: &KernelVersion,
		Os:            &Os,
		Version:       &Version,
	}
	return misc.NewGetVersionOK().WithPayload(version)
}

func (handlers *MiscHandlersImpl) Ping() middleware.Responder {
	return misc.NewPingOK().WithPayload("OK")
}