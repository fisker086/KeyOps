package aliyun

import (
	"strconv"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

func (a *AliyunAdapter) loadECSDetails() {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.RegionId = a.region
	request.PageSize = requests.NewInteger(MaxResults)

	pageNum := 1
	for {
		request.PageNumber = requests.NewInteger(pageNum)
		response, err := a.ecsClient.DescribeInstances(request)
		if err != nil {
			return
		}

		for _, instance := range response.Instances.Instance {
			a.instanceDetails[instance.InstanceId] = map[string]string{
				"InstanceName": instance.InstanceName,
				"ImageId":      instance.ImageId,
			}
		}

		if pageNum >= (response.TotalCount+MaxResults-1)/MaxResults {
			break
		}
		pageNum++
	}
}

func (a *AliyunAdapter) getSystemDiskIDs() (map[string]string, error) {
	request := ecs.CreateDescribeDisksRequest()
	request.Scheme = "https"
	request.RegionId = a.region
	request.PageSize = requests.NewInteger(MaxResults)

	result := make(map[string]string)
	pageNum := 1

	for {
		request.PageNumber = requests.NewInteger(pageNum)
		response, err := a.ecsClient.DescribeDisks(request)
		if err != nil {
			return result, err
		}

		for _, disk := range response.Disks.Disk {
			if disk.Device == "/dev/xvda" || disk.Device == "/dev/vda" {
				result[disk.InstanceId] = disk.DiskId
			}
		}

		if pageNum >= (response.TotalCount+MaxResults-1)/MaxResults {
			break
		}
		pageNum++
	}

	return result, nil
}

func (a *AliyunAdapter) getSnapshotChainUsage() (map[string]map[string]int64, map[string]int64, error) {
	request := ecs.CreateDescribeSnapshotsRequest()
	request.Scheme = "https"
	request.RegionId = a.region
	request.PageSize = requests.NewInteger(MaxResults)

	snapChainSize := make(map[string]map[string]int64)
	snapTotalSize := make(map[string]int64)

	pageNum := 1

	for {
		request.PageNumber = requests.NewInteger(pageNum)
		response, err := a.ecsClient.DescribeSnapshots(request)
		if err != nil {
			return snapChainSize, snapTotalSize, err
		}

		for _, snap := range response.Snapshots.Snapshot {
			sourceDiskID := snap.SourceDiskId
			if sourceDiskID == "" {
				continue
			}

			if _, exists := snapChainSize[a.region]; !exists {
				snapChainSize[a.region] = make(map[string]int64)
			}

			size, _ := strconv.ParseInt(snap.SourceDiskSize, 10, 64)
			snapChainSize[a.region][sourceDiskID] += size
			snapTotalSize[a.region] += size
		}

		if pageNum >= (response.TotalCount+MaxResults-1)/MaxResults {
			break
		}
		pageNum++
	}

	return snapChainSize, snapTotalSize, nil
}

func (a *AliyunAdapter) processSystemDiskItem(item map[string]interface{}, systemDiskIDs map[string]string) []map[string]interface{} {
	instanceID := a.getString(item, "InstanceID")
	diskID := systemDiskIDs[instanceID]

	result := make(map[string]interface{})
	for k, v := range item {
		result[k] = v
	}

	if diskID != "" {
		result["InstanceID"] = diskID
		result["ResourceId"] = diskID
		result["Tag"] = ""
		result["NickName"] = ""
	}

	productCode := "yundisk"
	result["ProductCode"] = productCode
	result["product_code"] = productCode
	result["ResourceType"] = "Disk"
	result["resource_type"] = "Disk"

	return []map[string]interface{}{result}
}

func (a *AliyunAdapter) processSnapshotItem(item map[string]interface{}, snapChainSize map[string]map[string]int64, snapTotalSize map[string]int64) []map[string]interface{} {
	regionCost := a.getFloat(item, "PretaxGrossAmount")

	regionChainSize, ok := snapChainSize[a.region]
	if !ok || len(regionChainSize) == 0 {
		return []map[string]interface{}{item}
	}

	regionTotal := snapTotalSize[a.region]
	if regionTotal == 0 {
		return []map[string]interface{}{item}
	}

	var results []map[string]interface{}
	costFields := []string{
		"PretaxGrossAmount", "PretaxAmount", "InvoiceDiscount", "DeductedByCoupons",
		"PendingAmount", "OutstandingAmount", "Amount", "Cost",
	}

	for chainID, size := range regionChainSize {
		chainItem := make(map[string]interface{})
		for k, v := range item {
			chainItem[k] = v
		}

		chainItem["InstanceID"] = chainID
		chainItem["ResourceId"] = chainID
		chainItem["Usage"] = float64(size)

		ratio := float64(size) / float64(regionTotal)
		chainItem["PretaxGrossAmount"] = regionCost * ratio

		for _, field := range costFields {
			if v := a.getFloat(item, field); v != 0 {
				chainItem[field] = v * ratio
			}
		}

		results = append(results, chainItem)
	}

	if len(results) == 0 {
		return []map[string]interface{}{item}
	}

	return results
}