def expand_with_co_authors(commit, pr, installation_id, commit_authors):
    """
    Helper to append UserCommitSummary objects for all co-authors to commit_authors list.
    """
    co_authors = cla.utils.get_co_authors_from_commit(commit)
    for co_author in co_authors:
        commit_authors.append(get_co_author_commits(co_author, commit, pr, installation_id))


def get_author_summary(commit, pr, installation_id) -> List[UserCommitSummary]:
    """
    Helper function to extract author information from a GitHub commit.
    :param commit: A GitHub commit object.
    :type commit: github.Commit.Commit
    :param pr: PR number
    :type pr: int
    """
    fn = "cla.models.github_models.get_author_summary"
    commit_authors = []
    if commit.author:
        try:
            commit_author_summary = UserCommitSummary(
                commit.sha,
                commit.author.id,
                commit.author.login,
                commit.author.name,
                commit.author.email,
                False,
                False,  # default not authorized - will be evaluated and updated later
            )
            cla.log.debug(f"{fn} - PR: {pr}, {commit_author_summary}")
            # check for co-author details
            # issue # 3884
            commit_authors.append(commit_author_summary)
            expand_with_co_authors(commit, pr, installation_id, commit_authors)
            return commit_authors
        except (GithubException, IncompletableObject) as exc:
            cla.log.warning(f"{fn} - PR: {pr}, unable to get commit author summary: {exc}")
            try:
                # commit.commit.author is a github.GitAuthor.GitAuthor object type - object
                # only has date, name and email attributes - no ID attribute/value
                # https://pygithub.readthedocs.io/en/latest/github_objects/GitAuthor.html
                commit_author_summary = UserCommitSummary(
                    commit.sha,
                    None,
                    None,
                    commit.commit.author.name,
                    commit.commit.author.email,
                    False,
                    False,  # default not authorized - will be evaluated and updated later
                )
                cla.log.debug(f"{fn} - github.GitAuthor.GitAuthor object: {commit.commit.author}")
                cla.log.debug(
                    f"{fn} - PR: {pr}, "
                    f"GitHub NamedUser author NOT found for commit SHA {commit_author_summary} "
                    f"however, we did find GitAuthor info"
                )
                cla.log.debug(f"{fn} - PR: {pr}, {commit_author_summary}")
                commit_authors.append(commit_author_summary)
                expand_with_co_authors(commit, pr, installation_id, commit_authors)
                return commit_authors
            except (GithubException, IncompletableObject) as exc:
                cla.log.warning(f"{fn} - PR: {pr}, unable to get commit author summary: {exc}")
                commit_author_summary = UserCommitSummary(commit.sha, None, None, None, None, False, False)
                cla.log.warning(f"{fn} - PR: {pr}, " f"could not find any commit author for SHA {commit_author_summary}")
                commit_authors.append(commit_author_summary)
                expand_with_co_authors(commit, pr, installation_id, commit_authors)
                return commit_authors
    else:
        cla.log.warning(f"{fn} - PR: {pr}, " f"could not find any commit author for SHA {commit.sha}")
        commit_author_summary = UserCommitSummary(commit.sha, None, None, None, None, False, False)
        commit_authors.append(commit_author_summary)
        expand_with_co_authors(commit, pr, installation_id, commit_authors)
        return commit_authors
