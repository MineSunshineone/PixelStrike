#!/bin/sh
set -eu

release_dir=${1:?release directory required}
app_dir=${2:?application directory required}
proxy=${3:?proxy path required}
openresty=${4:?openresty container required}
origin_host=${5:?origin host required}

source_dir="$release_dir/source"
[ -s "$release_dir/source.tar.gz" ]
mkdir -p "$source_dir"
tar -xzf "$release_dir/source.tar.gz" -C "$source_dir"
[ -s "$source_dir/docker-compose.prod.yml" ]
[ -s "$source_dir/deploy/1panel-root.conf" ]
cd "$app_dir"

cp docker-compose.yml docker-compose.yml.ci-rollback
cp "$proxy" "$proxy.ci-rollback"
server_rollback=0
client_rollback=0
if docker image inspect pixel-strike-server:latest >/dev/null 2>&1; then
  docker tag pixel-strike-server:latest pixel-strike-server:ci-rollback
  server_rollback=1
fi
if docker image inspect pixel-strike-client:latest >/dev/null 2>&1; then
  docker tag pixel-strike-client:latest pixel-strike-client:ci-rollback
  client_rollback=1
fi

rollback() {
  status=$?
  trap - EXIT HUP INT TERM
  if [ "$status" -ne 0 ]; then
    [ "$server_rollback" -eq 0 ] || docker tag pixel-strike-server:ci-rollback pixel-strike-server:latest
    [ "$client_rollback" -eq 0 ] || docker tag pixel-strike-client:ci-rollback pixel-strike-client:latest
    cp docker-compose.yml.ci-rollback docker-compose.yml
    cp "$proxy.ci-rollback" "$proxy"
    docker exec "$openresty" openresty -t && docker exec "$openresty" openresty -s reload || true
    docker compose unpause || true
    docker compose up -d --no-build --remove-orphans || true
  fi
  rm -f docker-compose.yml.ci-rollback "$proxy.ci-rollback"
  rm -rf "$release_dir"
  exit "$status"
}
trap rollback EXIT HUP INT TERM

docker build -f "$source_dir/server/Dockerfile" -t pixel-strike-server:latest "$source_dir"
docker build -f "$source_dir/client/Dockerfile" -t pixel-strike-client:latest "$source_dir"
tar -xzf "$release_dir/source.tar.gz" -C "$app_dir"
cp docker-compose.prod.yml docker-compose.yml
cp deploy/1panel-root.conf "$proxy"
docker exec "$openresty" openresty -t
docker exec "$openresty" openresty -s reload
docker compose unpause || true
docker compose up -d --no-build --remove-orphans --force-recreate

i=0
until curl -fsS http://127.0.0.1:12888/api/stats >/dev/null; do
  i=$((i + 1))
  [ "$i" -lt 20 ] || exit 1
  sleep 1
done
curl -fsS http://127.0.0.1:12888/map.json | grep -q '"size"'
asset=$(docker exec pixelstrike-prod-client sh -c 'basename /usr/share/nginx/html/assets/game-*.js')
curl -kfsS -H "Host: $origin_host" "https://127.0.0.1:4443/assets/$asset" >/dev/null

trap - EXIT HUP INT TERM
rm -rf "$release_dir"
rm -f docker-compose.yml.ci-rollback "$proxy.ci-rollback"
docker image rm pixel-strike-server:ci-rollback pixel-strike-client:ci-rollback >/dev/null 2>&1 || true
docker image prune -f >/dev/null
