package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const query = `
query UserRepos($login: String!, $after: String, $first: Int!) {
  user(login: $login) {
    repositories(
      first: 100
      after: $after
      orderBy: { field: PUSHED_AT, direction: DESC }
      ownerAffiliations: [OWNER]
      privacy: PUBLIC
    ) {
      totalCount
      pageInfo {
        hasNextPage
        endCursor
      }
      nodes {
        ...RepoFields
      }
    }
    pinnedItems(types: [REPOSITORY], first: $first) {
      nodes {
        ... on Repository {
          ...RepoFields
        }
      }
    }
  }
}

fragment RepoFields on Repository {
  createdAt
  description
  forkCount
  id
  name
  owner {
    login
    url
  }
  isArchived
  isFork
  licenseInfo {
    name
    nickname
    key
  }
  languages(first: 10, orderBy: { field: SIZE, direction: DESC }) {
    totalSize
    edges {
      size
      node {
        name
        color
      }
    }
  }
  stargazerCount
  url
}
`

const GITHUB_API_VERSION = "2026-03-10"

func FetchUserRepositories(client *http.Client, token string, login string, after *string) (Response, error) {
	body, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"login": login,
			"after": after,
			"first": 6,
		},
	})

	if err != nil {
		return Response{}, fmt.Errorf("marshal body: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(body))

	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}

	request.Header.Set("User-Agent", "nymph (github.com/ShadowFox88/nymph)")
	request.Header.Set("X-Github-Api-Version", GITHUB_API_VERSION)
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := client.Do(request)

	if err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)

	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	} else if response.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("unexpected response code: %d, %s", response.StatusCode, responseBody)
	}

	var rawResponse internalResponse
	if err := json.Unmarshal(responseBody, &rawResponse); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	output := rawResponse.toResponse()
	if dateHeader := response.Header.Get("Date"); dateHeader != "" {
		if parsedDate, err := http.ParseTime(dateHeader); err == nil {
			output.Date = parsedDate // otherwise, the Date is set when I call toResponse()
		}
	}

	return output, nil
}
