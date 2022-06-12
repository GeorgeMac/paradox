package main

import (
	"strings"

	"universe.dagger.io/docker"
	"dagger.io/dagger"
)

dagger.#Plan & {
	client: filesystem: {
		".": read: contents: dagger.#FS
	}

	#Base: {
		version: "1.18"
		packages: [pkgName=string]: version: string | *""
		// FIXME Remove once golang image include 1.18 *or* go compiler is smart with -buildvcs
		packages: {
			git: _
			// For GCC and other possible build dependencies
			"alpine-sdk": _
		}

		docker.#Build & {
			steps: [
				docker.#Pull & {
					source: "index.docker.io/golang:\(version)-alpine"
				},
				docker.#Set & {
					config: workdir: "/app"
				},
				for pkgName, pkg in packages {
					docker.#Run & {
						command: {
							name: "apk"
							args: ["add", "\(pkgName)\(pkg.version)"]
							flags: {
								"-U":         true
								"--no-cache": true
							}
						}
					}
				},
				for file in ["go.mod", "go.sum"] {
					docker.#Copy & {
						contents: client.filesystem.".".read.contents
						source:   file
						dest:     "/app/\(file)"
					}
				},
				docker.#Run & {
					command: {
						name: "go"
						args: ["mod", "download"]
					}
				},
				docker.#Copy & {
					contents: client.filesystem.".".read.contents
					dest:     "/app"
				},
				docker.#Run & {
					command: {
						name: "go"
						args: ["build", "./..."]
					}
				},
			]
		}
	}

	actions: {
		build: #Base

		_setupEnv: {
			name: "setup-envtest"
			args: ["use", "1.22"]
		}

		_setupEnvCall: "\(_setupEnv.name) \(strings.Join(_setupEnv.args, " "))"

		testdeps: docker.#Build & {
			steps: [
				#Base,
				docker.#Run & {
					command: {
						name: "go"
						args: ["install", "sigs.k8s.io/controller-runtime/tools/setup-envtest@latest"]
					}
				},
				docker.#Run & {
					command: _setupEnv
				},
			]
		}

		test: docker.#Run & {
			input: testdeps.output

			command: {
				name: "sh"
				args: ["-c", "KUBEBUILDER_ASSETS=$(\(_setupEnvCall) -p path) go test ./..."]
			}
		}
	}
}
