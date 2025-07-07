# IBM Packer Plugin - Post-Processors

## Scope
The IBM Packer Plugin's export-image post processor can be used to export images on IBM Cloud.

## Description
IBM Cloud's export-image post-processor will allow a customer to export any VPC custom image to a COS bucket; When below conditions are followed: 
  -   User Authorization
  -   Image fall in same region and is in available state (with status other than pending, failed, or deleting.

Generated **Image** from **Packer Builder** will be exported to **IBM COS Bucket** optionally **image_id** can also be specified in the post processor block for exporting. Also note if **image_id** is passed then **api_key** and **region** is mandatory.

### Post-Processor
- [ibmcloud-export-image](post-processor/ibmcloud-export-image) - The `ibmcloud-export-image` post-processor supports exporting custom images to COS bucket. 

### Prerequisites
Please refer to [README.md](https://github.com/IBM/packer-plugin-ibmcloud/blob/master/README.md) file from the main repository section.

## Usage

Follow [usage section](https://github.com/IBM/packer-plugin-ibmcloud/blob/master/README.md#:~:text=export%20OBJC_DISABLE_INITIALIZE_FORK_SAFETY%3DYES-,Usage,-Using%20the%20packer) of the README.md file from the main repository till step 3 and follow the below step 4.

4. Finally, run Packer plugin commands
    ```shell
    packer validate examples/postprocessor.vpc.rhel.pkr.hcl
    packer build examples/postprocessor.vpc.rhel.pkr.hcl
    ```

***********

## Packer Template in detail
Follow `Packer Template in detail` section of the [README.md](https://github.com/IBM/packer-plugin-ibmcloud/blob/master/README.md) and refer here for the post processor template.

```hcl
build {
  sources = [
    "source.ibmcloud-vpc.centos"
  ]
  post-processors {
    post-processor "ibmcloud-export-image" {
      image_export_job_name = "image-export-packer-1"
      storage_bucket_name   = "storage-bucket-1"
      format                = "qcow2"
    }
    post-processor "ibmcloud-export-image" {
      image_id              = "r006-47ce771b-2de5-4f63-9606-c9b7421d6888"
      image_export_job_name = "image-export-packer-2"
      api_key               = " "
      region                = "us-south"
      storage_bucket_name   = "storage-bucket-2"
      format                = "qcow2"
    }
    post-processor "ibmcloud-export-image" {
      image_id              = "r006-47ce771b-2de5-4f63-9606-c9b7421d6888"
      image_export_job_name = "image-export-packer-3"
      api_key               = " "
      region                = "us-south"
      storage_bucket_name   = "storage-bucket-2"
      format                = "qcow2"
    }
  }
}

```

### Understanding Packer Template Blocks

#### `post-processor` Block
The `variable` block defines variables within your Packer configuration. Input variables serve as parameters for a Packer build, allowing aspects of the build to be customized without altering the build's own source code. When you declare variables in the build of your configuration, you can set their values using CLI options and environment variables.

Variable | Type |Description
--- | --- | ---
**post-processor export image variables** |
| |
api_key | string | The IBM Cloud platform API key. Required only if image_id is provided.
export_timeout | string | The time to wait for export job to succeed. The format of this value is a duration such as "5s" or "5m".
region | string | IBM Cloud region where VPC is deployed. Required only if image_id is provided.
vpc_endpoint_url | string | Configure URL for VPC test environments. Optional.
iam_url | string | Configure URL for IAM test environments. Optional.
format | string | The format to use for the exported image. If the image is encrypted, only qcow2 is supported. Only [ qcow2, vhd ] are supported. Optional.
image_id | string | The image identifier to export image. If unspecified builder image_id will be used. Optional. 
image_export_job_name | string | The name for this image export job. Optional.
storage_bucket_name | string | The Cloud Object Storage bucket to export the image to. The bucket must exist and an IAM service authorization must grant Image Service for VPC of VPC Infrastructure Services writer access to the bucket. Required.

***********



