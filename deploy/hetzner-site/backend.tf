terraform {
  backend "s3" {
    key          = "leapview/site/production.tfstate"
    use_lockfile = true

    # Hetzner Object Storage is S3-compatible but does not expose the AWS
    # identity and metadata APIs. Bucket, region, endpoint, and credentials are
    # supplied by the protected workflow during terraform init.
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    skip_s3_checksum            = true
  }
}
