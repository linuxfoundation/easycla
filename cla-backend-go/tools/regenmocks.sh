# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#!/bin/bash

mkdir -p repositories/mock
mkdir -p events/mock
mkdir -p github_organizations/mock

# interfaces
mockgen -copyright_file=copyright-header.txt -source=repositories/service.go -destination=repositories/mock/mock_service.go -package=mock
mockgen -copyright_file=copyright-header.txt -source=repositories/repository.go -destination=repositories/mock/mock_repository.go -package=mock 
mockgen -copyright_file=copyright-header.txt -source=github_organizations/repository.go -destination=github_organizations/mock/mock_repository.go -package=mock RepositoryInterface
mockgen -copyright_file=copyright-header.txt -source=events/service.go -destination=events/mock/mock_service.go -package=mock Service
mockgen -copyright_file=copyright-header.txt -source=events/repository.go -destination=events/mock/mock_repository.go -package=mock RepositoryInterface