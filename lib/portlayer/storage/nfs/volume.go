// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package nfs

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
)

const (
	// The directory created in the NFS VolumeStore which we create volumes under
	volumesDir = "volumes"

	// path that namespaces the metadata for a specific volume. It lives beside the Volumes Directory.
	metadataDir = "volumes_metadata"

	// Stock permissions that are set, In the future we may pass these in.
	defaultPermissions = 0755

	DefaultUID = 1000

	nfsFilesystemTypeString = "nfs"
)

// VolumeStore this is nfs related volume store definition
type VolumeStore struct {
	// volume store name
	Name string

	// Service is the interface to the nfs target.
	Service MountServer

	// Service selflink to volume store.
	SelfLink *url.URL
}

func NewVolumeStore(op trace.Operation, storeName string, mount MountServer) (*VolumeStore, error) {
	u, _ := mount.URL()
	op.Infof("Creating nfs volumestore %s on %s", storeName, u.String())

	target, err := mount.Mount(op)
	if err != nil {
		return nil, err
	}
	defer mount.Unmount(op)

	selfLink, err := util.VolumeStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}

	v := &VolumeStore{
		Name:     storeName,
		Service:  mount,
		SelfLink: selfLink,
	}

	// we assume that nfsTargetURL.path already exists.
	// make volumes directory
	if _, err := target.Mkdir(volumesDir, defaultPermissions); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// make metadata directory
	if _, err := target.Mkdir(metadataDir, defaultPermissions); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return v, nil
}

// Returns the path to the vol relative to the given store.  The dir structure
// for a vol in a nfs store is `<configured nfs server path>/volumes/<vol ID>/<volume contents>`.
func (v *VolumeStore) volDirPath(ID string) string {
	return path.Join(volumesDir, ID)
}

// Returns the path to the metadata directory for a volume
func (v *VolumeStore) volMetadataDirPath(ID string) string {
	return path.Join(metadataDir, ID)
}

// Creates a volume directory and volume object for NFS based volumes
func (v *VolumeStore) VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*storage.Volume, error) {
	target, err := v.Service.Mount(op)
	if err != nil {
		return nil, err
	}
	defer v.Service.Unmount(op)

	if _, err := target.Mkdir(v.volDirPath(ID), defaultPermissions); err != nil {
		return nil, err
	}

	u, _ := v.Service.URL()
	if u.Scheme != nfsFilesystemTypeString {
		op.Errorf("URL from nfs mount target had scheme (%s) instead of nfs for volume store (%s)", u.Scheme, v.Name)
		return nil, fmt.Errorf("Unexpected scheme (%s) for volume store (%s)", u.Scheme, v.Name)
	}

	vol, err := storage.NewVolume(v.SelfLink, ID, info, NewVolume(u, v.volDirPath(ID)))
	if err != nil {
		return nil, err
	}

	if err := v.writeMetadata(op, ID, info, target); err != nil {
		return nil, err
	}

	op.Infof("nfs volume (%s) successfully created on volume store (%s)", ID, v.Name)
	return vol, nil
}

// Removes a volume and all of its contents from the nfs store. We already know via the cache if it is in use.
func (v *VolumeStore) VolumeDestroy(op trace.Operation, vol *storage.Volume) error {
	target, err := v.Service.Mount(op)
	if err != nil {
		return err
	}
	defer v.Service.Unmount(op)

	op.Infof("Attempting to remove volume (%s) and its metadata from volume store (%s)", vol.ID, v.Name)

	// remove volume directory and children
	if err := target.RemoveAll(v.volDirPath(vol.ID)); err != nil {
		op.Errorf("failed to remove volume (%s) on volume store (%s) due to error (%s)", vol.ID, v.Name, err)
		return err
	}

	// remove volume metadata directory and children
	if err := target.RemoveAll(v.volMetadataDirPath(vol.ID)); err != nil {
		op.Errorf("failed to remove metadata for volume (%s) at path (%q) on volume store (%s)", vol.ID, v.volDirPath(vol.ID), v.Name)
	}
	op.Infof("Successfully removed volume (%s) from volumestore (%s)", vol.ID, v.Name)

	return nil
}

func (v *VolumeStore) VolumesList(op trace.Operation) ([]*storage.Volume, error) {

	target, err := v.Service.Mount(op)
	if err != nil {
		return nil, err
	}
	defer v.Service.Unmount(op)

	volFileInfo, err := target.ReadDir(volumesDir)
	if err != nil {
		return nil, err
	}
	var volumes []*storage.Volume
	var fetchErr error

	for _, fileInfo := range volFileInfo {

		if fileInfo.Name() == "." || fileInfo.Name() == ".." {
			continue
		}

		volMetadata, err := v.getMetadata(op, fileInfo.Name(), target)
		if err != nil {
			op.Errorf("getting metadata for %s: %s", fileInfo.Name(), err.Error())
			fetchErr = err
			continue
		}

		u, _ := v.Service.URL()
		vol, err := storage.NewVolume(v.SelfLink, fileInfo.Name(), volMetadata, NewVolume(u, v.volDirPath(fileInfo.Name())))
		if err != nil {
			op.Errorf("Failed to create volume struct from volume directory (%s)", fileInfo.Name())
			return nil, err
		}

		volumes = append(volumes, vol)
	}

	if fetchErr != nil {
		return nil, err
	}

	return volumes, nil
}

func (v *VolumeStore) writeMetadata(op trace.Operation, ID string, info map[string][]byte, target Target) error {
	// write metadata into the metadata directory by key (filename) / value
	// (data), namespaced by volume id
	//
	// <root>/volume_matadata/<id>/<key>
	metadataPath := v.volMetadataDirPath(ID)

	_, err := target.Mkdir(metadataPath, defaultPermissions)
	if err != nil {
		return err
	}

	op.Infof("Writing metadata to (%s)", metadataPath)
	for fileName, data := range info {
		targetPath := path.Join(metadataPath, fileName)
		blobFile, err := target.OpenFile(targetPath, defaultPermissions)
		if err != nil {
			op.Errorf("openning file %s: %s", targetPath, err.Error())
			return err
		}
		defer blobFile.Close()

		_, err = blobFile.Write(data)
		if err != nil {
			return err
		}
		defer blobFile.Close()
	}
	op.Infof("Successfully wrote metadata to (%s)", metadataPath)
	return nil
}

func (v *VolumeStore) getMetadata(op trace.Operation, ID string, target Target) (map[string][]byte, error) {
	metadataPath := v.volMetadataDirPath(ID)
	op.Debugf("Attempting to retrieve volume metadata for (%s) at (%s)", ID, metadataPath)

	dataKeys, err := target.ReadDir(metadataPath)
	if err != nil {
		op.Errorf("readdir(%s): %s", metadataPath, err.Error())
		return nil, err
	}

	info := make(map[string][]byte)
	for _, keyFile := range dataKeys {

		if keyFile.Name() == "." || keyFile.Name() == ".." {
			continue
		}

		pth := path.Join(metadataPath, keyFile.Name())

		f, err := target.Open(pth)
		if err != nil {
			op.Errorf("open(%s): %s", pth, err.Error())
			return nil, err
		}
		defer f.Close()

		dataBlob, err := ioutil.ReadAll(f)
		if err != nil {
			op.Errorf("readall(%s): %s", pth, err.Error())
			return nil, err
		}

		info[keyFile.Name()] = dataBlob
	}

	op.Infof("Successfully read volume metadata at (%s)", metadataPath)
	return info, nil
}
