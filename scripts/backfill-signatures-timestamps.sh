#!/bin/bash

# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

set -euo pipefail

# Signature Timestamp Backfill Script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BINARY_NAME="signatures-timestamp-backfill"
BINARY_PATH="${PROJECT_ROOT}/bin/${BINARY_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

OPTIONS:
    -s, --stage STAGE       Target stage (dev, staging, prod) [required]
    -d, --dry-run          Run in dry-run mode (no changes made)
    -b, --build-only       Only build the binary, don't run it
    -f, --force            Force rebuild even if binary exists
    --allow-current-time   Allow using current time as fallback (default: false)
    -h, --help             Show this help message

ENVIRONMENT:
    AWS_PROFILE            AWS profile with access to cla-{stage}-signatures table [required]
    
EXAMPLES:
    $0 --stage staging --dry-run
    $0 --stage prod
    $0 --stage prod --allow-current-time
    $0 --build-only

EOF
}

validate_prerequisites() {
    log_info "Validating prerequisites..."
    
    if [[ ! -f "${PROJECT_ROOT}/go.mod" ]]; then
        log_error "Must be run from easycla cla-backend-go directory"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi
    
    if [[ "${BUILD_ONLY}" != "true" ]]; then
        if [[ -z "${AWS_PROFILE:-}" ]]; then
            log_error "AWS_PROFILE environment variable is required"
            exit 1
        fi
        
        if ! command -v aws &> /dev/null; then
            log_error "AWS CLI is not installed"
            exit 1
        fi
        
        if ! aws sts get-caller-identity --profile "${AWS_PROFILE}" &>/dev/null; then
            log_error "AWS profile '${AWS_PROFILE}' is not valid"
            exit 1
        fi
        
        log_info "AWS Profile: ${AWS_PROFILE}"
    fi
}

build_binary() {
    log_info "Building signature timestamp backfill utility..."
    cd "${PROJECT_ROOT}"
    mkdir -p bin
    
    if go build -ldflags="-s -w" -o "${BINARY_PATH}" ./cmd/signatures_timestamp_backfill; then
        log_success "Binary built successfully: ${BINARY_PATH}"
        chmod +x "${BINARY_PATH}"
    else
        log_error "Failed to build binary"
        exit 1
    fi
}

run_backfill() {
    log_info "Running signature timestamp backfill..."
    log_info "Stage: ${STAGE}"
    log_info "Dry Run: ${DRY_RUN}"
    log_info "Allow Current Time: ${ALLOW_CURRENT_TIME}"
    
    # Export environment variables for the Go binary
    export STAGE="${STAGE}"
    export DRY_RUN="${DRY_RUN}"
    export ALLOW_CURRENT_TIME="${ALLOW_CURRENT_TIME}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_warn "DRY RUN MODE - No changes will be made"
    fi
    
    if [[ "${ALLOW_CURRENT_TIME}" == "true" ]]; then
        log_warn "ALLOW CURRENT TIME - Will use current timestamp as fallback"
    else
        log_info "Current time fallback DISABLED - Will skip signatures without signed_on/docusign dates"
    fi
    
    local table_name="cla-${STAGE}-signatures"
    log_info "Target table: ${table_name}"
    
    if AWS_PROFILE="${AWS_PROFILE}" "${BINARY_PATH}"; then
        log_success "Backfill completed successfully"
    else
        local exit_code=$?
        log_error "Backfill failed with exit code: ${exit_code}"
        exit ${exit_code}
    fi
}

main() {
    local STAGE=""
    local DRY_RUN="false"
    local BUILD_ONLY="false"
    local FORCE_BUILD="false"
    local ALLOW_CURRENT_TIME="false"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -s|--stage)
                STAGE="$2"
                shift 2
                ;;
            -d|--dry-run)
                DRY_RUN="true"
                shift
                ;;
            -b|--build-only)
                BUILD_ONLY="true"
                shift
                ;;
            -f|--force)
                FORCE_BUILD="true"
                shift
                ;;
            --allow-current-time)
                ALLOW_CURRENT_TIME="true"
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    if [[ "${BUILD_ONLY}" != "true" ]] && [[ -z "${STAGE}" ]]; then
        log_error "Stage is required (use -s or --stage)"
        usage
        exit 1
    fi
    
    if [[ -n "${STAGE}" ]] && [[ ! "${STAGE}" =~ ^(dev|staging|prod)$ ]]; then
        log_error "Stage must be one of: dev, staging, prod"
        exit 1
    fi
    
    log_info "Starting signature timestamp backfill process..."
    validate_prerequisites
    
    if [[ "${FORCE_BUILD}" == "true" ]] || [[ ! -f "${BINARY_PATH}" ]]; then
        build_binary
    else
        log_info "Binary already exists: ${BINARY_PATH}"
    fi
    
    if [[ "${BUILD_ONLY}" != "true" ]]; then
        run_backfill
    else
        log_success "Build completed. Binary available at: ${BINARY_PATH}"
    fi
}

main "$@"
