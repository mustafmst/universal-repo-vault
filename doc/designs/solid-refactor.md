# SOLID code refactor

After getting to a state of working as a code spagetti I need to refactor this solution so it is easier to add new way of working like different cryptograpthic algorithms or archival methods

## Plan

1. Create vault object that will be initialized with a repo path that:
   - will load vauld data and lockfile data (maybe combine them together)
   - config provider injected
   - get cypher and archiver builder injected and then build those based on config
1. Create archivere adn archiver builder
1. create cypher and cypher builder that kas key injected
