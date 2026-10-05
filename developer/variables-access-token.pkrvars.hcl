# Variables file for IAM Service API Key token exchange authentication.
# Use with: packer build -var-file=developer/variables-access-token.pkrvars.hcl \
#             developer/examples/build.vpc.access-token.centos.pkr.hcl
#
# IAM_SERVICE_API_KEY — API key belonging to the Service ID that will act as the
#                       build identity. The plugin exchanges it for a scoped token
#                       internally on every VPC service initialisation.
# DESIRED_IAM_ID      — the CRN of the service identity the scoped token should
#                       be tied to (iam-authz desired_iam_id)

IAM_SERVICE_API_KEY    = ""
DESIRED_IAM_ID     = ""

SUBNET_ID         = ""
REGION            = ""
RESOURCE_GROUP_ID = ""
SECURITY_GROUP_ID = ""
