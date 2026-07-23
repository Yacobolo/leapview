package app

import (
	"net/http"

	apicapabilities "github.com/Yacobolo/leapview/internal/api/capabilities"
	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
)

func (a apiGenAdapter) GetCapabilities(w http.ResponseWriter, _ *http.Request) {
	apicapabilities.Write(w, apicapabilities.Config{
		Environment: a.server.defaultEnvironment,
		TUS:         a.server.managedDataTus != nil,
		S3Multipart: a.server.managedDataModule != nil && a.server.managedDataModule.Multipart() != nil,
	})
}

func (a apiGenAdapter) CreateRelease(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateReleaseHeaders) {
	a.server.releaseModule.CreateRelease(w, r, project, headers)
}

func (a apiGenAdapter) ListReleases(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListReleasesParams) {
	a.server.releaseModule.ListReleases(w, r, project, params)
}

func (a apiGenAdapter) GetRelease(w http.ResponseWriter, r *http.Request, project, releaseID string) {
	a.server.releaseModule.GetRelease(w, r, project, releaseID)
}

func (a apiGenAdapter) UploadReleaseArtifact(w http.ResponseWriter, r *http.Request, project, releaseID, workspaceID string, headers apigenapi.GenUploadReleaseArtifactHeaders) {
	a.server.releaseModule.UploadReleaseArtifact(w, r, project, releaseID, workspaceID, headers)
}

func (a apiGenAdapter) FinalizeRelease(w http.ResponseWriter, r *http.Request, project, releaseID string, headers apigenapi.GenFinalizeReleaseHeaders) {
	a.server.releaseModule.FinalizeRelease(w, r, project, releaseID, headers)
}

func (a apiGenAdapter) ListReleaseEvents(w http.ResponseWriter, r *http.Request, project, releaseID string, params apigenapi.GenListReleaseEventsParams, headers apigenapi.GenListReleaseEventsHeaders) {
	a.server.releaseModule.ListReleaseEvents(w, r, project, releaseID, params, headers)
}
