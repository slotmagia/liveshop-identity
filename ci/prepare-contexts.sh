#!/usr/bin/env bash
set -Eeuo pipefail

: "${CI_PROJECT_DIR:?CI_PROJECT_DIR is required}"
: "${CI_SERVER_URL:?CI_SERVER_URL is required}"
: "${CI_PROJECT_NAMESPACE:?CI_PROJECT_NAMESPACE is required}"
: "${CI_JOB_TOKEN:?CI_JOB_TOKEN is required}"

workspace_root="$(cd "$(dirname "$CI_PROJECT_DIR")" && pwd -P)"
server_without_scheme="${CI_SERVER_URL#*://}"
authenticated_base="${CI_SERVER_PROTOCOL}://gitlab-ci-token:${CI_JOB_TOKEN}@${server_without_scheme}"

for repository in ${DEPENDENCY_REPOSITORIES:-}; do
  case "$repository" in
    kernel-go|liveshop-platform|liveshop-identity|liveshop-gateway|liveshop-catalog|liveshop-trade|liveshop-live) ;;
    *) printf 'Unsupported dependency repository: %s\n' "$repository" >&2; exit 1 ;;
  esac

  target="$workspace_root/$repository"
  resolved_parent="$(cd "$(dirname "$target")" && pwd -P)"
  if [ "$resolved_parent" != "$workspace_root" ] || [ "$target" = "$CI_PROJECT_DIR" ]; then
    printf 'Unsafe dependency target: %s\n' "$target" >&2
    exit 1
  fi

  rm -rf -- "$target"
  git clone --quiet --depth 1     "$authenticated_base/$CI_PROJECT_NAMESPACE/$repository.git"     "$target"
  git -C "$target" remote set-url origin "$CI_SERVER_URL/$CI_PROJECT_NAMESPACE/$repository.git"
  printf 'Prepared dependency %s\n' "$repository"
done

