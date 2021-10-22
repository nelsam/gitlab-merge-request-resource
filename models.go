package resource

import (
	"net/url"
	"os"
	"regexp"
	"time"
)

type Source struct {
	URI                string   `json:"uri"`
	PrivateToken       string   `json:"private_token"`
	PrivateKey         string   `json:"private_key"`
	Insecure           bool     `json:"insecure"`
	SkipWorkInProgress bool     `json:"skip_work_in_progress,omitempty"`
	SkipNotMergeable   bool     `json:"skip_not_mergeable,omitempty"`
	SkipTriggerComment bool     `json:"skip_trigger_comment,omitempty"`
	ConcourseUrl       string   `json:"concourse_url,omitempty"`
	PipelineName       string   `json:"pipeline_name,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	TargetBranch       string   `json:"target_branch,omitempty"`
	Submodules         string   `json:"submodules"`
	SubmoduleCreds     []struct {
		Host     string `json:"host"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"submodule_credentials"`
}

type Version struct {
	ID        int        `json:"id,string"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type Metadata []MetadataField

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GetBaseURL extracts host from URI (repository URL) and appends the v3 API suffix.
func (source *Source) GetBaseURL() string {
	r, _ := regexp.Compile("https?://[^/]+")
	host := r.FindString(source.URI)
	return host + "/api/v4"
}

// GetGitlabServerDomain extracts the Gitlab Server domain from URI.
func (source *Source) GetGitlabServerDomain() string {
	r, _ := regexp.Compile("(https?|ssh)://([^/]*)/(.*)\\.git$")
	return r.FindStringSubmatch(source.URI)[2]
}

// GetProjectPath extracts project path from URI (repository URL).
func (source *Source) GetProjectPath() string {
	r, _ := regexp.Compile("(https?|ssh)://([^/]*)/(.*)\\.git$")
	return r.FindStringSubmatch(source.URI)[3]
}

func (source *Source) GetTargetURL() string {
	target, _ := url.Parse(source.GetCoucourseUrl())
	target.Path += "/teams/" + url.QueryEscape(os.Getenv("BUILD_TEAM_NAME"))
	target.Path += "/pipelines/" + url.QueryEscape(os.Getenv("BUILD_PIPELINE_NAME"))
	target.Path += "/jobs/" + url.QueryEscape(os.Getenv("BUILD_JOB_NAME"))
	target.Path += "/builds/" + url.QueryEscape(os.Getenv("BUILD_NAME"))
	return target.String()
}

func (source *Source) GetCoucourseUrl() string {
	if source.ConcourseUrl != "" {
		return source.ConcourseUrl
	} else {
		return os.Getenv("ATC_EXTERNAL_URL")
	}
}

func (source *Source) GetPipelineName() string {
	if source.PipelineName != "" {
		return source.PipelineName
	} else {
		return os.Getenv("BUILD_PIPELINE_NAME")
	}

}
