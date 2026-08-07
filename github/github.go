package github

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
