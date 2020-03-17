/*
Copyright 2018 OpenEBS Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	apis "github.com/openebs/node-disk-manager/pkg/apis/openebs/v1alpha1"
	udev "github.com/openebs/node-disk-manager/pkg/udev"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DiskInfo contains details of a disk which can be converted into api.Disk
// This is one utility struct used by different module like probe, controller
// also in event message. One DiskInfo struct for each disk is created when
// one event is generated then each probe fills that disk related data in
// that struct. At the end it is converted to Disk struct which will push to
// etcd as a CR of that disk.
type DiskInfo struct {
	ProbeIdentifiers      ProbeIdentifier // ProbeIdentifiers contains some keys to uniquely identify each disk by probe
	NodeAttributes        NodeAttribute   // NodeAttribute contains the node's attributes like hostname and nodename
	Uuid                  string          // Uuid is the unique id given by ndm
	Capacity              uint64          // Capacity is capacity of a disk
	Model                 string          // Model is model no of a disk
	Serial                string          // Serial is serial no of a disk
	Vendor                string          // Vendor is vendor of a disk
	Path                  string          // Path is dev path of a disk like /dev/sda
	ByIdDevLinks          []string        // ByIdDevLinks contains by-id devlinks
	ByPathDevLinks        []string        // ByPathDevLinks contains by-path devlinks
	FirmwareRevision      string          // FirmwareRevision is the firmware revision for a disk
	LogicalSectorSize     uint32          // LogicalSectorSize is the Logical size of disk sector in bytes
	PhysicalSectorSize    uint32          // PhysicalSectorSize is the Physical size of disk sector in bytes
	RotationRate          uint16          // 0 = not reported. 1 = SSD, everything else is an RPM
	Compliance            string          // Compliance is implemented specifications version i.e. SPC-1, SPC-2, etc
	DiskType              string          // DiskType represents the type of disk like Disk, Sparse etc.,
	DriveType             string          // DriveType represents the type of disk like HHD, HDD etc.,
	FileSystemInformation FSInfo          // FileSystemInformation stores the FS related information like filesystem type and mountpoint
	PartitionData         []PartitionInfo // Information of the partitions on the disk

	//Stats of disk which keep changing
	TotalBytesRead        uint64
	TotalBytesWritten     uint64
	DeviceUtilizationRate float64
	PercentEnduranceUsed  float64
	//Temperature of Drive, all in degree celsius
	TemperatureInfo struct {
		TemperatureDataValid bool
		CurrentTemperature   int16
		HighestValid         bool
		HighestTemperature   int16 //lifetime measured highest
		LowestValid          bool
		LowestTemperature    int16 //lifetime measured lowest
	}
}

// NodeAttribute is a map of string, which stores various attributes like hostname, node name, failure domain
// etc of a node
type NodeAttribute map[string]string

// ProbeIdentifier contains some keys to enable probes to uniquely identify each disk.
// These keys are defined here in order to denote the identifier that a particular probe
// needs in order to identify a particular disk such as device Path for smart probe and
// syspath for udev probe
// Defining all the identifiers separately here makes it more clear and readable to know
// the keys or the fields that a particular probe requires to identify each and every disk
// Uuid here is totally related to udev probe since udev is the only probe to scan disks and
// as a part of discovery, it assign uuid to each disk and which is copied to uuid field of
// DiskInfo struct.
type ProbeIdentifier struct {
	Uuid               string // Uuid is uuid of disk which is generated by udev probe.
	UdevIdentifier     string // UdevIdentifier(syspath) used to identify disk by udevprobe.
	SmartIdentifier    string // SmartIdentifier (devPath) is used to identify disk by smartprobe.
	SeachestIdentifier string // SeachestIdentifier (devPath) is used to identify disk by seachest.
	MountIdentifier    string // MountIdentifier (devPath) is used to identify disks by mountprobe
}

// PartitionInfo defines the partition related information like partition type, filesystem etc
// on the partition
type PartitionInfo struct {
	PartitionType         string // Partition type like 83, 8e etc.
	FileSystemInformation FSInfo // FileSystem related information like, filesystem type, mountpoint
}

// FSInfo defines the filesystem related information of block device/disk, like mountpoint and
// filesystem
type FSInfo struct {
	FileSystem string // Filesystem on the block device
	MountPoint string // MountPoint of the block device
}

// NewDiskInfo returns a pointer of empty diskInfo struct which will
// be field by different probes each probe will responsible for
// populate some specific fields of DiskInfo struct.
func NewDiskInfo() *DiskInfo {
	nodeAttribute := make(NodeAttribute)
	diskInfo := &DiskInfo{
		NodeAttributes: nodeAttribute,
	}
	diskInfo.DiskType = NDMDefaultDiskType
	return diskInfo
}

// ToDisk convert diskInfo struct to api.Disk type which will be pushed to etcd
func (di *DiskInfo) ToDisk() apis.Disk {
	dr := apis.Disk{}
	dr.TypeMeta = di.getTypeMeta()
	dr.ObjectMeta = di.getObjectMeta()
	dr.Spec = di.getDiskSpec()
	dr.Status = di.getStatus()
	dr.Stats = di.getStats()
	return dr
}

// ToPartition convert the PartitionData struct inside DiskInfo to apis.Partition
// which will be used to include the partition information in the Disk CR
func (di *DiskInfo) ToPartition() []apis.Partition {
	partition := make([]apis.Partition, 0)
	for _, partitionData := range di.PartitionData {
		partition = append(partition, apis.Partition{PartitionType: partitionData.PartitionType,
			FileSystem: partitionData.FileSystemInformation.getFileSystemInfo()})
	}
	return partition
}

// getObjectMeta returns ObjectMeta struct which contains labels and Name of resource
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getObjectMeta() metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{
		Labels: make(map[string]string),
		Name:   di.Uuid,
	}
	objectMeta.Labels[KubernetesHostNameLabel] = di.NodeAttributes[HostNameKey]
	objectMeta.Labels[NDMDiskTypeKey] = di.DiskType
	objectMeta.Labels[NDMManagedKey] = TrueString
	return objectMeta
}

// getTypeMeta returns TypeMeta struct which contains resource kind and version
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getTypeMeta() metav1.TypeMeta {
	typeMeta := metav1.TypeMeta{
		Kind:       NDMDiskKind,
		APIVersion: NDMVersion,
	}
	return typeMeta
}

// getStatus returns DiskStatus struct which contains state of resource
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getStatus() apis.DiskStatus {
	diskStatus := apis.DiskStatus{
		State: NDMActive,
	}
	return diskStatus
}

// getDiskSpec returns DiskSpec struct which contains info of disk
// like - static details - (model,serial,vendor ..)
// capacity - (size,logical sector size ...)
// devlinks - (by-id , by-path links)
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getDiskSpec() apis.DiskSpec {
	diskSpec := apis.DiskSpec{}
	diskSpec.Path = di.getPath()
	diskSpec.Capacity = di.getDiskCapacity()
	diskSpec.Details = di.getDiskDetails()
	diskSpec.DevLinks = di.getDiskLinks()
	diskSpec.FileSystem = di.FileSystemInformation.getFileSystemInfo()
	return diskSpec
}

// getPath returns path of the disk like (/dev/sda , /dev/sdb ...)
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getPath() string {
	return di.Path
}

// getDiskDetails returns DiskDetails struct which contains primary and static info of
// disk resource like model, serial, vendor .. these data must present for each disk
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getDiskDetails() apis.DiskDetails {
	diskDetails := apis.DiskDetails{}
	diskDetails.Model = di.Model
	diskDetails.Serial = di.Serial
	diskDetails.Vendor = di.Vendor
	diskDetails.FirmwareRevision = di.FirmwareRevision
	diskDetails.Compliance = di.Compliance
	diskDetails.DriveType = di.DriveType
	diskDetails.RotationRate = di.RotationRate
	return diskDetails
}

// getDiskCapacity returns DiskCapacity struct which contains size of disk
// size contains only total size for now later we will add logical, physical
// sector size of a disk in this struct.
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getDiskCapacity() apis.DiskCapacity {
	capacity := apis.DiskCapacity{}
	capacity.Storage = di.Capacity
	capacity.LogicalSectorSize = di.LogicalSectorSize
	capacity.PhysicalSectorSize = di.PhysicalSectorSize
	return capacity
}

// getDiskLinks returns slice of DiskDevLink struct which contains soft links
// like by-id ,by-path link
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getDiskLinks() []apis.DiskDevLink {
	devLinks := make([]apis.DiskDevLink, 0)
	if len(di.ByIdDevLinks) != 0 {
		byIdLinks := apis.DiskDevLink{
			Kind:  "by-id",
			Links: di.ByIdDevLinks,
		}
		devLinks = append(devLinks, byIdLinks)
	}
	if len(di.ByPathDevLinks) != 0 {
		byPathLinks := apis.DiskDevLink{
			Kind:  "by-path",
			Links: di.ByPathDevLinks,
		}
		devLinks = append(devLinks, byPathLinks)
	}
	return devLinks
}

// getDiskSpec returns DiskSpec struct which contains info of disk
// like - static details - (model,serial,vendor ..)
// capacity - (size,logical sector size ...)
// devlinks - (by-id , by-path links)
// It is used to populate data of Disk struct which is a disk CR.
func (di *DiskInfo) getStats() apis.DiskStat {
	diskStat := apis.DiskStat{}
	diskStat.TotalBytesRead = di.TotalBytesRead
	diskStat.TotalBytesWritten = di.TotalBytesWritten
	diskStat.DeviceUtilizationRate = di.DeviceUtilizationRate
	diskStat.PercentEnduranceUsed = di.PercentEnduranceUsed
	if di.TemperatureInfo.TemperatureDataValid == true {
		diskStat.TempInfo.CurrentTemperature = di.TemperatureInfo.CurrentTemperature
		diskStat.TempInfo.HighestTemperature = di.TemperatureInfo.HighestTemperature
		diskStat.TempInfo.LowestTemperature = di.TemperatureInfo.LowestTemperature
	}
	return diskStat
}

func (fs *FSInfo) getFileSystemInfo() apis.FileSystemInfo {
	fsInfo := apis.FileSystemInfo{}
	if fs.FileSystem != udev.UDEV_FS_NONE {
		fsInfo.Type = fs.FileSystem
		fsInfo.Mountpoint = fs.MountPoint
	}
	return fsInfo
}