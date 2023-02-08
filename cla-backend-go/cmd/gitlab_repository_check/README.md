# GitLab Repository Check Lambda

GitLab (currently) does not support sending callback/webhook events for GitLab project add or delete events. As a
result, we created a small lambda that runs periodically to check for any new GitLab project
(repository) add or deletes.

The process/algorithm is:

1. Query our database for registered GitLab Groups - filter by the enabled flag is true and where the Auto Enable flag
   is true
1. For each GitLab group in our database...
    1. Create a new GitLab API client instance using the authorization token for the Git Group
    1. Query the GitLab API for the project list under the group (include sub-groups). This grabs the list of current
       GitLab projects under the GitLab group.
    1. Query for GitLab project in DB matching this GitLab group path
    1. Identify deltas - this identifies how many new and deleted GitLap projects we need to process
    1. If any new GitLab projects, add to the DB, set enabled, create an event log
    1. If any removed/deleted GitLab projects, remove from the DB, create an event log

## References

- [GitLab Feature request discussion thread](https://gitlab.com/gitlab-com/marketing/community-relations/opensource-program/linux-foundation/-/issues/4#note_653255564)
