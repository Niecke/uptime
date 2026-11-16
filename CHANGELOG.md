# Changelog

## [0.2.1](https://github.com/Niecke/uptime/compare/v0.2.0...v0.2.1) (2026-11-16)


### Bug Fixes

* bump Go to 1.27.4 in go.mod and the golang image ([38e5760](https://github.com/Niecke/uptime/commit/38e5760ec6da7e7750a328a401df2cf125d81ee2))
* **caddy:** exclude /events from encode with a request matcher ([93a322d](https://github.com/Niecke/uptime/commit/93a322d5f96899d8868c3f8ba2f37577632652a5))
* **caddy:** exclude /events from encode with a request matcher and validate the Caddyfile in CI ([a9e223f](https://github.com/Niecke/uptime/commit/a9e223f2cb74a7dc1b29dbdb4ccba3513b4679ad))
* **db:** repair escaped backticks in list_endpoints_test.go ([a815846](https://github.com/Niecke/uptime/commit/a81584614df59bdf611ba3c999b166765e666268))
* **deps:** update module github.com/go-chi/chi/v5 to v5.4.0 [security] ([274deb3](https://github.com/Niecke/uptime/commit/274deb3cc2c5b02d747e6360f5de75bb5432a718))
* **deps:** update module github.com/go-chi/chi/v5 to v5.4.0 [security] ([8601271](https://github.com/Niecke/uptime/commit/86012711fa4380e4f187ac9d869b15f5397d1f78))
* **deps:** update module github.com/go-chi/chi/v5 to v5.5.0 ([026deb5](https://github.com/Niecke/uptime/commit/026deb5a58402ebfb2e0673a3e13e1b7becb197f))
* **deps:** update module github.com/go-chi/chi/v5 to v5.5.0 ([474e114](https://github.com/Niecke/uptime/commit/474e1148e36b5b1952a3aba8aef30b790279c717))
* **deps:** update module github.com/pressly/goose/v3 to v3.30.1 ([613587d](https://github.com/Niecke/uptime/commit/613587de1f9e741b25ba9a3c8a05c4555f186d2a))
* **deps:** update module github.com/pressly/goose/v3 to v3.30.1 ([4d37a29](https://github.com/Niecke/uptime/commit/4d37a2970d0175aeba642709399a5cbd9c285f70))
* **renovate:** bump go.mod go directive together with the golang image ([5f73df3](https://github.com/Niecke/uptime/commit/5f73df3798c5176e7ff7bfb77a66a5379272a332))
* **renovate:** bump go.mod go directive together with the golang image ([ea35bf7](https://github.com/Niecke/uptime/commit/ea35bf73e8717bce32c934f75a717d6bc09b61b6))


### Performance Improvements

* **db:** enable incremental auto_vacuum and return freed pages after retention ([b2d11f7](https://github.com/Niecke/uptime/commit/b2d11f74208da91bdcb5ef79891108f8c7879c0d))
* **db:** enable incremental auto_vacuum so retention deletes shrink the file ([33c30a0](https://github.com/Niecke/uptime/commit/33c30a050e51289ed9bdeb6414326afc46157f84))
* **db:** look up the latest check per endpoint with an index seek ([7bd942e](https://github.com/Niecke/uptime/commit/7bd942ec1d27f51782c741c22c453588eb97ba65))
* **db:** look up the latest check per endpoint with an index seek ([6bea4b2](https://github.com/Niecke/uptime/commit/6bea4b2d0f73e5e92ee904de2ae590180d8d12b7))

## [0.2.0](https://github.com/Niecke/uptime/compare/v0.1.7...v0.2.0) (2026-11-07)


### Features

* **api:** serve robots.txt and add noindex meta tag ([b00c628](https://github.com/Niecke/uptime/commit/b00c62882a58b980de700f58d4503a1ab8e78494))
* **caddy:** send X-Robots-Tag noindex, nofollow ([2a207e2](https://github.com/Niecke/uptime/commit/2a207e250bc81e1fa42967c5f87f1ca8a1a3b701))
* keep the dashboard out of search engines with noindex and robots.txt ([f9e4c58](https://github.com/Niecke/uptime/commit/f9e4c58d15d7fe3d906c7288f7fc921895e12b30))
* new release pipeline ([0728b6a](https://github.com/Niecke/uptime/commit/0728b6a0d15a87446c3fdfa6f19bdd440138c139))


### Bug Fixes

* bump Go to 1.27.1 so govulncheck passes ([0f4f1e0](https://github.com/Niecke/uptime/commit/0f4f1e09dc36b74b49d7c88911cd2015af5138aa))
* bump Go to 1.27.1 so govulncheck passes ([c21cc6b](https://github.com/Niecke/uptime/commit/c21cc6b56659700c9b73a05ec2c7560f445d18f2))
* bump Go to 1.27.2 so govulncheck passes ([858b008](https://github.com/Niecke/uptime/commit/858b0087c01497dbe1272517e245eecf944334c5))
* bump Go to 1.27.3 so govulncheck passes ([74b00a1](https://github.com/Niecke/uptime/commit/74b00a1087c86d8679dc12a12f60a41aaf3fbd11))
* **deps:** update module github.com/pressly/goose/v3 to v3.29.0 ([be98656](https://github.com/Niecke/uptime/commit/be98656c3a6c8baa42a762afe5b66d8bec40a838))
* **deps:** update module github.com/pressly/goose/v3 to v3.29.0 ([5bfd325](https://github.com/Niecke/uptime/commit/5bfd325bf736194a441513624daa4921781341ef))
* **deps:** update module modernc.org/sqlite to v1.61.0 ([63a3813](https://github.com/Niecke/uptime/commit/63a38139838a7e0e200d1712f23836975126c2a7))
* **deps:** update module modernc.org/sqlite to v1.61.0 ([45c36df](https://github.com/Niecke/uptime/commit/45c36df41b4cc4f3fec5e124c6e2d597cbeaabe5))
* **deps:** update module modernc.org/sqlite to v1.61.1 ([57f635d](https://github.com/Niecke/uptime/commit/57f635d89547d7be4556d2ecc002191a9a488d4d))
* **deps:** update module modernc.org/sqlite to v1.61.1 ([42d99b1](https://github.com/Niecke/uptime/commit/42d99b13bcaeb09a5dc46f01aa0f635539a21a73))
* **deps:** update module modernc.org/sqlite to v1.62.0 ([04f6cbe](https://github.com/Niecke/uptime/commit/04f6cbe4c955c00b7b170c921e79e923b62c9023))
* **deps:** update module modernc.org/sqlite to v1.62.0 ([f7df7ed](https://github.com/Niecke/uptime/commit/f7df7ed821954fbaa71e20a70564d6182f6eda5e))
* select the pull request that merged the commit when promoting ([92afdd4](https://github.com/Niecke/uptime/commit/92afdd4c67bfbcbb211d3728ba867846d910391d))

## Changelog
