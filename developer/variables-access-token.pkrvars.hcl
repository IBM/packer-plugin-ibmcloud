# Variables file for IAM token exchange authentication.
# Use with: packer build -var-file=developer/variables-access-token.pkrvars.hcl \
#             developer/examples/build.vpc.access-token.centos.pkr.hcl
#
# IAM_ACCESS_TOKEN    — your existing IBM Cloud access token (e.g. from `ibmcloud iam oauth-tokens`)
# IAM_DESIRED_IAM_ID  — the CRN of the service identity the scoped token should be tied to
# IAM_TOKEN_EXCHANGE_URL — optional; defaults to https://iam.cloud.ibm.com/identity/token

IAM_ACCESS_TOKEN       = ""
IAM_DESIRED_IAM_ID     = "crn:v1:bluemix:public:cloud-object-storage:global:a/<account_id>:<instance_id>::"
IAM_TOKEN_EXCHANGE_URL = "https://iam.cloud.ibm.com/identity/token"

SUBNET_ID         = ""
REGION            = ""
RESOURCE_GROUP_ID = ""
SECURITY_GROUP_ID = ""
