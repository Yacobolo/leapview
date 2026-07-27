package app

import (
	appconfig "github.com/Yacobolo/leapview/internal/app/config"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
)

func managedDataProductConfig(config appconfig.Config) manageddatamodule.ProductConfig {
	return manageddatamodule.ProductConfig{
		Backend:           config.ManagedDataBackend,
		Dir:               config.ManagedDataDir,
		MaxFiles:          config.ManagedDataMaxFiles,
		MaxFileBytes:      config.ManagedDataMaxFileBytes,
		MaxRevisionBytes:  config.ManagedDataMaxRevisionBytes,
		MinFreeBytes:      config.ManagedDataMinFreeBytes,
		UploadSessionTTL:  config.ManagedDataUploadSessionTTL,
		GCGracePeriod:     config.ManagedDataGCGracePeriod,
		S3Region:          config.ManagedDataS3Region,
		S3AccessKeyID:     config.ManagedDataS3AccessKeyID,
		S3SecretAccessKey: config.ManagedDataS3SecretAccessKey,
		S3SessionToken:    config.ManagedDataS3SessionToken,
		S3PathStyle:       config.ManagedDataS3PathStyle,
		S3Endpoint:        config.ManagedDataS3Endpoint,
		S3Bucket:          config.ManagedDataS3Bucket,
		S3Prefix:          config.ManagedDataS3Prefix,
	}
}
