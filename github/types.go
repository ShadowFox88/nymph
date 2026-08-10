package github

import "time"

type Response struct {
	Repositories       []Repository
	PinnedRepositories []Repository
	TotalCount         int
	HasNextPage        bool
	EndCursor          string
	Date               time.Time
}

type Repository struct {
	CreatedAt      time.Time
	Description    string
	Forks          int
	ID             string
	Name           string
	Owner          Owner
	IsArchived     bool
	IsFork         bool
	License        *License
	Languages      []Language
	StargazerCount int
	URL            string
}

type Owner struct {
	Name string
	URL  string
}

type License struct {
	Name     string
	Nickname string
	Key      string
}

type Language struct {
	Name       string
	Colour     string
	Size       int
	Percentage float64
}

func (r internalResponse) toResponse() Response {
	return Response{
		Repositories:       toRepositories(r.Data.User.ResponseRepositories.Repositories),
		PinnedRepositories: toRepositories(r.Data.User.PinnedRepositories.Repositories),
		TotalCount:         r.Data.User.ResponseRepositories.TotalCount,
		HasNextPage:        r.Data.User.ResponseRepositories.PageInfo.HasNextPage,
		EndCursor:          r.Data.User.ResponseRepositories.PageInfo.EndCursor,
		Date:               time.Now(),
	}
}

func toRepositories(repos []internalRepository) []Repository {
	output := []Repository{}

	for _, repo := range repos {
		description := ""
		if repo.Description != nil {
			description = *repo.Description
		}
		output = append(output, Repository{
			CreatedAt:      repo.CreatedAt,
			Description:    description,
			Forks:          repo.Forks,
			ID:             repo.ID,
			Name:           repo.Name,
			Owner:          Owner{Name: repo.Owner.Name, URL: repo.Owner.URL},
			IsArchived:     repo.IsArchived,
			IsFork:         repo.IsFork,
			License:        toLicense(repo.LicenseInfo),
			Languages:      toLanguages(repo.Languages),
			StargazerCount: repo.StargazerCount,
			URL:            repo.URL,
		})
	}

	return output
}

func toLicense(license *internalLicense) *License {
	if license == nil {
		return nil
	}

	nickname := ""
	if license.Nickname != nil {
		nickname = *license.Nickname
	}

	return &License{
		Name:     license.Name,
		Nickname: nickname,
		Key:      license.Key,
	}
}

func toLanguages(languages internalLanguages) []Language {
	edges := languages.Edges
	if len(edges) == 0 {
		return nil
	} // should only happen for like empty repos

	output := []Language{}

	for _, edge := range edges {
		var percentage float64 = 0
		if languages.TotalSize > 0 {
			percentage = (float64(edge.Size) / float64(languages.TotalSize))
		}
		output = append(output, Language{
			Name:       edge.Language.Name,
			Colour:     edge.Language.Colour,
			Size:       edge.Size,
			Percentage: float64(percentage),
		})
	}

	return output
}

type internalResponse struct {
	Data internalData `json:"data"`
}

type internalData struct {
	User internalResponseUser `json:"user"`
}

type internalResponseUser struct {
	ResponseRepositories internalResponseRepositories `json:"repositories"`
	PinnedRepositories   internalPinnedRepositories   `json:"pinnedItems"`
}

type internalResponseRepositories struct {
	TotalCount   int                  `json:"totalCount"`
	PageInfo     internalPageInfo     `json:"pageInfo"`
	Repositories []internalRepository `json:"nodes"`
}

type internalPinnedRepositories struct {
	Repositories []internalRepository `json:"nodes"`
}

type internalPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type internalRepository struct {
	CreatedAt      time.Time         `json:"createdAt"`
	Description    *string           `json:"description"`
	Forks          int               `json:"forkCount"`
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Owner          internalOwner     `json:"owner"`
	IsArchived     bool              `json:"isArchived"`
	IsFork         bool              `json:"isFork"`
	LicenseInfo    *internalLicense  `json:"licenseInfo"`
	Languages      internalLanguages `json:"languages"`
	StargazerCount int               `json:"stargazerCount"`
	URL            string            `json:"url"`
}

type internalLicense struct {
	Name     string  `json:"name"`
	Nickname *string `json:"nickname"`
	Key      string  `json:"key"`
}

type internalOwner struct {
	Name string `json:"login"`
	URL  string `json:"url"`
}

type internalLanguages struct {
	TotalSize int            `json:"totalSize"`
	Edges     []internalEdge `json:"edges"`
}

type internalEdge struct {
	Size     int              `json:"size"`
	Language internalLanguage `json:"node"`
}

type internalLanguage struct {
	Name   string `json:"name"`
	Colour string `json:"color"`
}
