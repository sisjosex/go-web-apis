# Tools for the offline geo build (INFRA-003): osmium for the places export, node for
# the style JSON. Routing tiles and the basemap have images of their own (build.sh).
FROM node:22-bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends osmium-tool curl ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /opt/geo
RUN npm install --omit=dev @protomaps/basemaps@5.7.2
COPY style.mjs places.json ./
