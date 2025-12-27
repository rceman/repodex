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
			"coverage/",
			".cache/",
			"package-lock.json",
			"npm-debug.log*",
			"yarn-debug.log*",
			"yarn-error.log*",
			"pnpm-debug.log*",
			".DS_Store",
			"**/*.map",
		},
	}
}
