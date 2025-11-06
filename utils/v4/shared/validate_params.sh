#!/bin/bash
# Shared parameter validation functions for V4 APIs

# Validate UUID v4 format
validate_uuid_v4() {
    local uuid="$1"
    local param_name="$2"
    
    if [[ ! $uuid =~ ^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$ ]]; then
        echo "Error: ${param_name} '${uuid}' is not a valid UUID v4"
        return 1
    fi
    return 0
}

# Validate SFID format (15 or 18 characters, alphanumeric)
validate_sfid() {
    local sfid="$1"
    local param_name="$2"
    
    if [[ ! $sfid =~ ^[0-9A-Za-z]{15}$|^[0-9A-Za-z]{18}$ ]]; then
        echo "Error: ${param_name} '${sfid}' is not a valid SFID (must be 15 or 18 alphanumeric characters)"
        return 1
    fi
    return 0
}

# Validate email format
validate_email() {
    local email="$1"
    local param_name="$2"
    
    if [[ ! $email =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
        echo "Error: ${param_name} '${email}' is not a valid email address"
        return 1
    fi
    return 0
}

# Validate URL format
validate_url() {
    local url="$1"
    local param_name="$2"
    
    if [[ ! $url =~ ^https?://[^[:space:]]+$ ]]; then
        echo "Error: ${param_name} '${url}' is not a valid URL"
        return 1
    fi
    return 0
}

# Check if parameter is not empty
validate_not_empty() {
    local value="$1"
    local param_name="$2"
    
    if [ -z "$value" ]; then
        echo "Error: ${param_name} cannot be empty"
        return 1
    fi
    return 0
}