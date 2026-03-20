#!/bin/bash

# Optimus Server Deploy Script
# Builds and pushes optimus-server Docker image to DigitalOcean registry

set -e

# Colors
GREEN='\033[38;2;39;201;63m'
YELLOW='\033[38;2;222;184;65m'
BLUE='\033[38;2;59;130;246m'
GRAY='\033[38;2;136;136;136m'
RED='\033[0;31m'
NC='\033[0m'

# Registry configuration
REGISTRY="registry.digitalocean.com"
REPO="abrayall"
IMAGE="optimus-server"

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo "=============================================="
echo -e "${YELLOW}Optimus Server Deploy${NC}"
echo "=============================================="
echo ""

# Get version using vermouth
VERSION=$(vermouth 2>/dev/null || curl -sfL https://raw.githubusercontent.com/abrayall/vermouth/refs/heads/main/vermouth.sh | sh -)

# Full image names
VERSION_TAG="${REGISTRY}/${REPO}/${IMAGE}:${VERSION}"
LATEST_TAG="${REGISTRY}/${REPO}/${IMAGE}:latest"

echo -e "${BLUE}Version:${NC}  ${VERSION}"
echo -e "${BLUE}Registry:${NC} ${REGISTRY}/${REPO}"
echo -e "${BLUE}Image:${NC}    ${IMAGE}"
echo ""

# Build the Docker image
echo -e "${YELLOW}Building Docker image...${NC}"
echo ""

docker build \
    --platform linux/amd64 \
    --build-arg VERSION="${VERSION}" \
    -t "${VERSION_TAG}" \
    -t "${LATEST_TAG}" \
    .

echo ""
echo -e "${GREEN}✓ Built: ${VERSION_TAG}${NC}"
echo ""

# Login to registry
TOKEN="${DIGITALOCEAN_TOKEN:-$TOKEN}"
if [ -n "$TOKEN" ]; then
    echo -e "${BLUE}Logging in to registry...${NC}"
    echo "$TOKEN" | docker login "$REGISTRY" --username "$TOKEN" --password-stdin
    echo ""
else
    echo -e "${GRAY}No DIGITALOCEAN_TOKEN env var set, assuming already logged in${NC}"
fi

# Push to registry
echo -e "${YELLOW}Pushing to registry...${NC}"
echo ""

docker push "${VERSION_TAG}"
docker push "${LATEST_TAG}"

echo ""
echo -e "${GREEN}✓ Pushed images${NC}"
echo ""

# Check/create DigitalOcean App
APP_NAME="optimus-server"
PROJECT_NAME="optimus"

if [ -z "$TOKEN" ]; then
    echo -e "${GRAY}No DIGITALOCEAN_TOKEN set, skipping app deployment${NC}"
else
    echo -e "${YELLOW}Checking DigitalOcean App Platform...${NC}"

    # Get all apps and search for our app by name
    APPS_RESPONSE=$(curl -s -X GET \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        "https://api.digitalocean.com/v2/apps")

    # Check if our app exists
    if echo "$APPS_RESPONSE" | grep -q "\"name\":\"$APP_NAME\""; then
        echo -e "${GREEN}✓ App '$APP_NAME' already exists${NC}"

        APP_URL=$(echo "$APPS_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for app in data.get('apps', []):
    if app.get('spec', {}).get('name') == '$APP_NAME':
        print(app.get('live_url', ''))
        break
" 2>/dev/null)

        if [ -n "$APP_URL" ]; then
            echo -e "${BLUE}  URL:${NC} $APP_URL"
        fi
        echo -e "${GRAY}  Deployment will be triggered automatically by deploy_on_push${NC}"
    else
        echo -e "${BLUE}Creating app '$APP_NAME'...${NC}"

        # Build app spec with env vars from GitHub secrets
        APP_SPEC=$(cat <<SPEC
{"spec":{"name":"optimus-server","region":"nyc","features":["buildpack-stack=ubuntu-22"],"alerts":[{"rule":"DEPLOYMENT_FAILED"},{"rule":"DOMAIN_FAILED"}],"ingress":{"rules":[{"component":{"name":"optimus-server"},"match":{"path":{"prefix":"/"}}}]},"services":[{"name":"optimus-server","http_port":8080,"image":{"registry_type":"DOCR","registry":"abrayall","repository":"optimus-server","tag":"latest","deploy_on_push":{"enabled":true}},"health_check":{"http_path":"/api/health","initial_delay_seconds":30,"period_seconds":10,"timeout_seconds":3,"success_threshold":1,"failure_threshold":3},"instance_count":1,"instance_size_slug":"apps-s-1vcpu-1gb","envs":[{"key":"CLAUDE_TOKEN","value":"${CLAUDE_TOKEN}","type":"SECRET"},{"key":"AWS_ACCESS_KEY_ID","value":"${AWS_ACCESS_KEY_ID}","type":"SECRET"},{"key":"AWS_SECRET_ACCESS_KEY","value":"${AWS_SECRET_ACCESS_KEY}","type":"SECRET"},{"key":"S3_BUCKET","value":"${S3_BUCKET}","type":"GENERAL"},{"key":"S3_ENDPOINT","value":"${S3_ENDPOINT}","type":"GENERAL"},{"key":"SERP_API_KEY","value":"${SERP_API_KEY}","type":"SECRET"}]}]}}
SPEC
)

        RESPONSE=$(curl -s -X POST \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "$APP_SPEC" \
            "https://api.digitalocean.com/v2/apps")

        if echo "$RESPONSE" | grep -q '"app"'; then
            echo -e "${GREEN}✓ App '$APP_NAME' created${NC}"

            APP_ID=$(echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('app', {}).get('id', ''))
" 2>/dev/null)

            # Assign app to project
            if [ -n "$PROJECT_NAME" ]; then
                PROJECT_ID=$(curl -s -X GET \
                    -H "Authorization: Bearer $TOKEN" \
                    -H "Content-Type: application/json" \
                    "https://api.digitalocean.com/v2/projects" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for p in data.get('projects', []):
    if p.get('name') == '$PROJECT_NAME':
        print(p.get('id', ''))
        break
" 2>/dev/null)

                if [ -n "$PROJECT_ID" ]; then
                    curl -s -X POST \
                        -H "Authorization: Bearer $TOKEN" \
                        -H "Content-Type: application/json" \
                        -d "{\"resources\":[\"do:app:$APP_ID\"]}" \
                        "https://api.digitalocean.com/v2/projects/$PROJECT_ID/resources" > /dev/null
                    echo -e "${GREEN}✓ Assigned to project '$PROJECT_NAME'${NC}"
                else
                    echo -e "${YELLOW}⚠ Project '$PROJECT_NAME' not found, app in default project${NC}"
                fi
            fi

            echo -e "${GRAY}  Waiting for deployment...${NC}"

            # Poll for deployment status
            LAST_STATUS=""
            TIMEOUT=300
            ELAPSED=0

            while [ $ELAPSED -lt $TIMEOUT ]; do
                STATUS_RESPONSE=$(curl -s -X GET \
                    -H "Authorization: Bearer $TOKEN" \
                    -H "Content-Type: application/json" \
                    "https://api.digitalocean.com/v2/apps/$APP_ID")

                CURRENT_STATUS=$(echo "$STATUS_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
app = data.get('app', {})
deployment = app.get('active_deployment') or app.get('pending_deployment') or {}
print(deployment.get('phase', 'UNKNOWN'))
" 2>/dev/null)

                # Show status changes
                if [ "$CURRENT_STATUS" != "$LAST_STATUS" ]; then
                    case "$CURRENT_STATUS" in
                        "PENDING_BUILD") echo -e "${GRAY}  Status: Pending build...${NC}" ;;
                        "BUILDING") echo -e "${YELLOW}  Status: Building...${NC}" ;;
                        "PENDING_DEPLOY") echo -e "${GRAY}  Status: Pending deploy...${NC}" ;;
                        "DEPLOYING") echo -e "${YELLOW}  Status: Deploying...${NC}" ;;
                        "ACTIVE") echo -e "${GREEN}  Status: Active${NC}" ;;
                        "ERROR"|"FAILED") echo -e "${RED}  Status: Failed${NC}" ;;
                        *) echo -e "${GRAY}  Status: $CURRENT_STATUS${NC}" ;;
                    esac
                    LAST_STATUS="$CURRENT_STATUS"
                fi

                # Check for terminal states
                if [ "$CURRENT_STATUS" = "ACTIVE" ]; then
                    APP_URL=$(echo "$STATUS_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('app', {}).get('live_url', ''))
" 2>/dev/null)
                    echo ""
                    echo -e "${GREEN}✓ Deployment successful!${NC}"
                    if [ -n "$APP_URL" ]; then
                        echo -e "${BLUE}  URL:${NC} $APP_URL"
                    fi
                    break
                fi

                if [ "$CURRENT_STATUS" = "ERROR" ] || [ "$CURRENT_STATUS" = "FAILED" ]; then
                    echo ""
                    echo -e "${RED}✗ Deployment failed${NC}"
                    break
                fi

                sleep 5
                ELAPSED=$((ELAPSED + 5))
            done

            if [ $ELAPSED -ge $TIMEOUT ]; then
                echo -e "${YELLOW}⚠ Deployment still in progress (timed out waiting)${NC}"
            fi
        else
            echo -e "${RED}✗ Failed to create app${NC}"
            ERROR_MSG=$(echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('message', data.get('id', 'Unknown error')))
" 2>/dev/null || echo "$RESPONSE")
            echo -e "${RED}  Error: $ERROR_MSG${NC}"
        fi
    fi
fi

echo ""
echo "=============================================="
echo -e "${GREEN}Deploy Complete!${NC}"
echo "=============================================="
echo ""
echo "Pushed images:"
echo "  • ${VERSION_TAG}"
echo "  • ${LATEST_TAG}"
echo ""
