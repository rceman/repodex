package profile

import "os"

type nodeProfile struct{}

func newNodeProfile() Profile {
	return nodeProfile{}
}

func (nodeProfile) ID() string {
	return "node"
}

func (nodeProfile) Detect(ctx DetectContext) (bool, error) {
	_, err := os.Stat(ctx.Join("package.json"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (nodeProfile) Rules() Rules {
	return Rules{
		ScanIgnore: []string{
			"node_modules/",
			"dist/",
			"build/",
			"out/",
			".next/",
			".nuxt/",
			".svelte-kit/",
			".astro/",
			".vite/",
			".parcel-cache/",
			".turbo/",
			".vercel/",
			"coverage/",
			".nyc_output/",
			".cache/",
			"package-lock.json",
			"npm-shrinkwrap.json",
			"npm-debug.log*",
			"yarn-debug.log*",
			"yarn-error.log*",
			"yarn.lock",
			"pnpm-debug.log*",
			"pnpm-lock.yaml",
			"bun.lockb",
			".DS_Store",
			"**/*.map",
		},
	}
}
