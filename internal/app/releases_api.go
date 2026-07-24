package app

import (
	"net/http"

	apicapabilities "github.com/Yacobolo/leapview/internal/app/api/capabilities"
	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
)

func (a apiGenDispatcher) GetCapabilities(w http.ResponseWriter, _ *http.Request) {
	apicapabilities.Write(w, apicapabilities.Config{
		Environment: a.defaultEnvironment,
		TUS:         a.managedDataTus != nil,
		S3Multipart: a.managedDataModule != nil && a.managedDataModule.SupportsS3Multipart(),
	})
}

func (a apiGenDispatcher) CreateRelease(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateReleaseHeaders) {
	a.releaseModule.CreateRelease(w, r, project, headers.IdempotencyKey)
}

func (a apiGenDispatcher) ListReleases(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListReleasesParams) {
	a.releaseModule.ListReleases(w, r, project, releasemodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) GetRelease(w http.ResponseWriter, r *http.Request, project, releaseID string) {
	a.releaseModule.GetRelease(w, r, project, releaseID)
}

func (a apiGenDispatcher) UploadReleaseArtifact(w http.ResponseWriter, r *http.Request, project, releaseID, workspaceID string, headers apigenapi.GenUploadReleaseArtifactHeaders) {
	a.releaseModule.UploadReleaseArtifact(w, r, project, releaseID, workspaceID, headers.ContentType, headers.ContentDigest)
}

func (a apiGenDispatcher) FinalizeRelease(w http.ResponseWriter, r *http.Request, project, releaseID string, headers apigenapi.GenFinalizeReleaseHeaders) {
	a.releaseModule.FinalizeRelease(w, r, project, releaseID)
}

func (a apiGenDispatcher) ListReleaseEvents(w http.ResponseWriter, r *http.Request, project, releaseID string, params apigenapi.GenListReleaseEventsParams, headers apigenapi.GenListReleaseEventsHeaders) {
	a.releaseModule.ListReleaseEvents(w, r, project, releaseID, releasemodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}
