echo -e "\033[1;36mDeploying development environment...\033[0m"

echo -e "\033[1;32mPulling the latest Docker image...\033[0m"
docker pull ghcr.io/hcd233/aris-proxy-api:master

echo -e "\033[1;34mStarting up services with docker-compose...\033[0m"
docker compose -f docker/docker-compose-dev-single.yml up -d

echo -e "\033[1;31mPruning unused Docker images...\033[0m"
docker image prune -a -f

echo -e "\033[1;33mDisplaying Docker logs for aris-proxy-api...\033[0m"
docker logs -f aris-proxy-api-dev --details