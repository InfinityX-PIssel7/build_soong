// Copyright 2015 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package android

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestSrcIsModule(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name       string
		args       args
		wantModule string
	}{
		{
			name: "file",
			args: args{
				s: "foo",
			},
			wantModule: "",
		},
		{
			name: "module",
			args: args{
				s: ":foo",
			},
			wantModule: "foo",
		},
		{
			name: "tag",
			args: args{
				s: ":foo{.bar}",
			},
			wantModule: "foo{.bar}",
		},
		{
			name: "extra colon",
			args: args{
				s: ":foo:bar",
			},
			wantModule: "foo:bar",
		},
		{
			name: "fully qualified",
			args: args{
				s: "//foo:bar",
			},
			wantModule: "//foo:bar",
		},
		{
			name: "fully qualified with tag",
			args: args{
				s: "//foo:bar{.tag}",
			},
			wantModule: "//foo:bar{.tag}",
		},
		{
			name: "invalid unqualified name",
			args: args{
				s: ":foo/bar",
			},
			wantModule: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotModule := SrcIsModule(tt.args.s); gotModule != tt.wantModule {
				t.Errorf("SrcIsModule() = %v, want %v", gotModule, tt.wantModule)
			}
		})
	}
}

func TestSrcIsModuleWithTag(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name       string
		args       args
		wantModule string
		wantTag    string
	}{
		{
			name: "file",
			args: args{
				s: "foo",
			},
			wantModule: "",
			wantTag:    "",
		},
		{
			name: "module",
			args: args{
				s: ":foo",
			},
			wantModule: "foo",
			wantTag:    "",
		},
		{
			name: "tag",
			args: args{
				s: ":foo{.bar}",
			},
			wantModule: "foo",
			wantTag:    ".bar",
		},
		{
			name: "empty tag",
			args: args{
				s: ":foo{}",
			},
			wantModule: "foo",
			wantTag:    "",
		},
		{
			name: "extra colon",
			args: args{
				s: ":foo:bar",
			},
			wantModule: "foo:bar",
		},
		{
			name: "invalid tag",
			args: args{
				s: ":foo{.bar",
			},
			wantModule: "foo{.bar",
		},
		{
			name: "invalid tag 2",
			args: args{
				s: ":foo.bar}",
			},
			wantModule: "foo.bar}",
		},
		{
			name: "fully qualified",
			args: args{
				s: "//foo:bar",
			},
			wantModule: "//foo:bar",
		},
		{
			name: "fully qualified with tag",
			args: args{
				s: "//foo:bar{.tag}",
			},
			wantModule: "//foo:bar",
			wantTag:    ".tag",
		},
		{
			name: "invalid unqualified name",
			args: args{
				s: ":foo/bar",
			},
			wantModule: "",
		},
		{
			name: "invalid unqualified name with tag",
			args: args{
				s: ":foo/bar{.tag}",
			},
			wantModule: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModule, gotTag := SrcIsModuleWithTag(tt.args.s)
			if gotModule != tt.wantModule {
				t.Errorf("SrcIsModuleWithTag() gotModule = %v, want %v", gotModule, tt.wantModule)
			}
			if gotTag != tt.wantTag {
				t.Errorf("SrcIsModuleWithTag() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

type depsModule struct {
	ModuleBase
	props struct {
		Deps []string
	}
}

func (m *depsModule) GenerateAndroidBuildActions(ctx ModuleContext) {
	outputFile := PathForModuleOut(ctx, ctx.ModuleName())
	ctx.Build(pctx, BuildParams{
		Rule:   Touch,
		Output: outputFile,
	})
	installFile := ctx.InstallFile(PathForModuleInstall(ctx), ctx.ModuleName(), outputFile)
	ctx.InstallSymlink(PathForModuleInstall(ctx, "symlinks"), ctx.ModuleName(), installFile)
}

func (m *depsModule) DepsMutator(ctx BottomUpMutatorContext) {
	ctx.AddDependency(ctx.Module(), installDepTag{}, m.props.Deps...)
}

func depsModuleFactory() Module {
	m := &depsModule{}
	m.AddProperties(&m.props)
	InitAndroidArchModule(m, HostAndDeviceDefault, MultilibCommon)
	return m
}

var prepareForModuleTests = FixtureRegisterWithContext(func(ctx RegistrationContext) {
	ctx.RegisterModuleType("deps", depsModuleFactory)
})

func TestErrorDependsOnDisabledModule(t *testing.T) {
	bp := `
		deps {
			name: "foo",
			deps: ["bar"],
		}
		deps {
			name: "bar",
			enabled: false,
		}
	`

	prepareForModuleTests.
		ExtendWithErrorHandler(FixtureExpectsAtLeastOneErrorMatchingPattern(`module "foo": depends on disabled module "bar"`)).
		RunTestWithBp(t, bp)
}

func TestValidateCorrectBuildParams(t *testing.T) {
	config := TestConfig(t.TempDir(), nil, "", nil)
	pathContext := PathContextForTesting(config)
	bparams := convertBuildParams(BuildParams{
		// Test with Output
		Output:        PathForOutput(pathContext, "undeclared_symlink"),
		SymlinkOutput: PathForOutput(pathContext, "undeclared_symlink"),
	})

	err := validateBuildParams(bparams)
	if err != nil {
		t.Error(err)
	}

	bparams = convertBuildParams(BuildParams{
		// Test with ImplicitOutput
		ImplicitOutput: PathForOutput(pathContext, "undeclared_symlink"),
		SymlinkOutput:  PathForOutput(pathContext, "undeclared_symlink"),
	})

	err = validateBuildParams(bparams)
	if err != nil {
		t.Error(err)
	}
}

func TestValidateIncorrectBuildParams(t *testing.T) {
	config := TestConfig(t.TempDir(), nil, "", nil)
	pathContext := PathContextForTesting(config)
	params := BuildParams{
		Output:          PathForOutput(pathContext, "regular_output"),
		Outputs:         PathsForOutput(pathContext, []string{"out1", "out2"}),
		ImplicitOutput:  PathForOutput(pathContext, "implicit_output"),
		ImplicitOutputs: PathsForOutput(pathContext, []string{"i_out1", "_out2"}),
		SymlinkOutput:   PathForOutput(pathContext, "undeclared_symlink"),
	}

	bparams := convertBuildParams(params)
	err := validateBuildParams(bparams)
	if err != nil {
		FailIfNoMatchingErrors(t, "undeclared_symlink is not a declared output or implicit output", []error{err})
	} else {
		t.Errorf("Expected build params to fail validation: %+v", bparams)
	}
}

func TestDistErrorChecking(t *testing.T) {
	bp := `
		deps {
			name: "foo",
      dist: {
        dest: "../invalid-dest",
        dir: "../invalid-dir",
        suffix: "invalid/suffix",
      },
      dists: [
        {
          dest: "../invalid-dest0",
          dir: "../invalid-dir0",
          suffix: "invalid/suffix0",
        },
        {
          dest: "../invalid-dest1",
          dir: "../invalid-dir1",
          suffix: "invalid/suffix1",
        },
      ],
 		}
	`

	expectedErrs := []string{
		"\\QAndroid.bp:5:13: module \"foo\": dist.dest: Path is outside directory: ../invalid-dest\\E",
		"\\QAndroid.bp:6:12: module \"foo\": dist.dir: Path is outside directory: ../invalid-dir\\E",
		"\\QAndroid.bp:7:15: module \"foo\": dist.suffix: Suffix may not contain a '/' character.\\E",
		"\\QAndroid.bp:11:15: module \"foo\": dists[0].dest: Path is outside directory: ../invalid-dest0\\E",
		"\\QAndroid.bp:12:14: module \"foo\": dists[0].dir: Path is outside directory: ../invalid-dir0\\E",
		"\\QAndroid.bp:13:17: module \"foo\": dists[0].suffix: Suffix may not contain a '/' character.\\E",
		"\\QAndroid.bp:16:15: module \"foo\": dists[1].dest: Path is outside directory: ../invalid-dest1\\E",
		"\\QAndroid.bp:17:14: module \"foo\": dists[1].dir: Path is outside directory: ../invalid-dir1\\E",
		"\\QAndroid.bp:18:17: module \"foo\": dists[1].suffix: Suffix may not contain a '/' character.\\E",
	}

	prepareForModuleTests.
		ExtendWithErrorHandler(FixtureExpectsAllErrorsToMatchAPattern(expectedErrs)).
		RunTestWithBp(t, bp)
}

func TestInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}
	bp := `
		deps {
			name: "foo",
			deps: ["bar"],
		}

		deps {
			name: "bar",
			deps: ["baz", "qux"],
		}

		deps {
			name: "baz",
			deps: ["qux"],
		}

		deps {
			name: "qux",
		}
	`

	result := GroupFixturePreparers(
		prepareForModuleTests,
		PrepareForTestWithArchMutator,
	).RunTestWithBp(t, bp)

	module := func(name string, host bool) TestingModule {
		variant := "android_common"
		if host {
			variant = result.Config.BuildOSCommonTarget.String()
		}
		return result.ModuleForTests(name, variant)
	}

	outputRule := func(name string) TestingBuildParams { return module(name, false).Output(name) }

	installRule := func(name string) TestingBuildParams {
		return module(name, false).Output(filepath.Join("out/soong/target/product/test_device/system", name))
	}

	symlinkRule := func(name string) TestingBuildParams {
		return module(name, false).Output(filepath.Join("out/soong/target/product/test_device/system/symlinks", name))
	}

	hostOutputRule := func(name string) TestingBuildParams { return module(name, true).Output(name) }

	hostInstallRule := func(name string) TestingBuildParams {
		return module(name, true).Output(filepath.Join("out/soong/host/linux-x86", name))
	}

	hostSymlinkRule := func(name string) TestingBuildParams {
		return module(name, true).Output(filepath.Join("out/soong/host/linux-x86/symlinks", name))
	}

	assertInputs := func(params TestingBuildParams, inputs ...Path) {
		t.Helper()
		AssertArrayString(t, "expected inputs", Paths(inputs).Strings(),
			append(PathsIfNonNil(params.Input), params.Inputs...).Strings())
	}

	assertImplicits := func(params TestingBuildParams, implicits ...Path) {
		t.Helper()
		AssertArrayString(t, "expected implicit dependencies", Paths(implicits).Strings(),
			append(PathsIfNonNil(params.Implicit), params.Implicits...).Strings())
	}

	assertOrderOnlys := func(params TestingBuildParams, orderonlys ...Path) {
		t.Helper()
		AssertArrayString(t, "expected orderonly dependencies", Paths(orderonlys).Strings(),
			params.OrderOnly.Strings())
	}

	// Check host install rule dependencies
	assertInputs(hostInstallRule("foo"), hostOutputRule("foo").Output)
	assertImplicits(hostInstallRule("foo"),
		hostInstallRule("bar").Output,
		hostSymlinkRule("bar").Output,
		hostInstallRule("baz").Output,
		hostSymlinkRule("baz").Output,
		hostInstallRule("qux").Output,
		hostSymlinkRule("qux").Output,
	)
	assertOrderOnlys(hostInstallRule("foo"))

	// Check host symlink rule dependencies.  Host symlinks must use a normal dependency, not an
	// order-only dependency, so that the tool gets updated when the symlink is depended on.
	assertInputs(hostSymlinkRule("foo"), hostInstallRule("foo").Output)
	assertImplicits(hostSymlinkRule("foo"))
	assertOrderOnlys(hostSymlinkRule("foo"))

	// Check device install rule dependencies
	assertInputs(installRule("foo"), outputRule("foo").Output)
	assertImplicits(installRule("foo"))
	assertOrderOnlys(installRule("foo"),
		installRule("bar").Output,
		symlinkRule("bar").Output,
		installRule("baz").Output,
		symlinkRule("baz").Output,
		installRule("qux").Output,
		symlinkRule("qux").Output,
	)

	// Check device symlink rule dependencies.  Device symlinks could use an order-only dependency,
	// but the current implementation uses a normal dependency.
	assertInputs(symlinkRule("foo"), installRule("foo").Output)
	assertImplicits(symlinkRule("foo"))
	assertOrderOnlys(symlinkRule("foo"))
}

func TestInstallKatiEnabled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}
	bp := `
		deps {
			name: "foo",
			deps: ["bar"],
		}

		deps {
			name: "bar",
			deps: ["baz", "qux"],
		}

		deps {
			name: "baz",
			deps: ["qux"],
		}

		deps {
			name: "qux",
		}
	`

	result := GroupFixturePreparers(
		prepareForModuleTests,
		PrepareForTestWithArchMutator,
		FixtureModifyConfig(SetKatiEnabledForTests),
		PrepareForTestWithMakevars,
	).RunTestWithBp(t, bp)

	rules := result.InstallMakeRulesForTesting(t)

	module := func(name string, host bool) TestingModule {
		variant := "android_common"
		if host {
			variant = result.Config.BuildOSCommonTarget.String()
		}
		return result.ModuleForTests(name, variant)
	}

	outputRule := func(name string) TestingBuildParams { return module(name, false).Output(name) }

	ruleForOutput := func(output string) InstallMakeRule {
		for _, rule := range rules {
			if rule.Target == output {
				return rule
			}
		}
		t.Fatalf("no make install rule for %s", output)
		return InstallMakeRule{}
	}

	installRule := func(name string) InstallMakeRule {
		return ruleForOutput(filepath.Join("out/target/product/test_device/system", name))
	}

	symlinkRule := func(name string) InstallMakeRule {
		return ruleForOutput(filepath.Join("out/target/product/test_device/system/symlinks", name))
	}

	hostOutputRule := func(name string) TestingBuildParams { return module(name, true).Output(name) }

	hostInstallRule := func(name string) InstallMakeRule {
		return ruleForOutput(filepath.Join("out/host/linux-x86", name))
	}

	hostSymlinkRule := func(name string) InstallMakeRule {
		return ruleForOutput(filepath.Join("out/host/linux-x86/symlinks", name))
	}

	assertDeps := func(rule InstallMakeRule, deps ...string) {
		t.Helper()
		AssertArrayString(t, "expected inputs", deps, rule.Deps)
	}

	assertOrderOnlys := func(rule InstallMakeRule, orderonlys ...string) {
		t.Helper()
		AssertArrayString(t, "expected orderonly dependencies", orderonlys, rule.OrderOnlyDeps)
	}

	// Check host install rule dependencies
	assertDeps(hostInstallRule("foo"),
		hostOutputRule("foo").Output.String(),
		hostInstallRule("bar").Target,
		hostSymlinkRule("bar").Target,
		hostInstallRule("baz").Target,
		hostSymlinkRule("baz").Target,
		hostInstallRule("qux").Target,
		hostSymlinkRule("qux").Target,
	)
	assertOrderOnlys(hostInstallRule("foo"))

	// Check host symlink rule dependencies.  Host symlinks must use a normal dependency, not an
	// order-only dependency, so that the tool gets updated when the symlink is depended on.
	assertDeps(hostSymlinkRule("foo"), hostInstallRule("foo").Target)
	assertOrderOnlys(hostSymlinkRule("foo"))

	// Check device install rule dependencies
	assertDeps(installRule("foo"), outputRule("foo").Output.String())
	assertOrderOnlys(installRule("foo"),
		installRule("bar").Target,
		symlinkRule("bar").Target,
		installRule("baz").Target,
		symlinkRule("baz").Target,
		installRule("qux").Target,
		symlinkRule("qux").Target,
	)

	// Check device symlink rule dependencies.  Device symlinks could use an order-only dependency,
	// but the current implementation uses a normal dependency.
	assertDeps(symlinkRule("foo"), installRule("foo").Target)
	assertOrderOnlys(symlinkRule("foo"))
}

type PropsTestModuleEmbedded struct {
	Embedded_prop *string
}

type propsTestModule struct {
	ModuleBase
	DefaultableModuleBase
	props struct {
		A string `android:"arch_variant"`
		B *bool
		C []string
	}
	otherProps struct {
		PropsTestModuleEmbedded

		D      *int64
		Nested struct {
			E *string
		}
		F *string `blueprint:"mutated"`
	}
}

func propsTestModuleFactory() Module {
	module := &propsTestModule{}
	module.AddProperties(&module.props, &module.otherProps)
	InitAndroidArchModule(module, HostAndDeviceSupported, MultilibBoth)
	InitDefaultableModule(module)
	return module
}

type propsTestModuleDefaults struct {
	ModuleBase
	DefaultsModuleBase
}

func propsTestModuleDefaultsFactory() Module {
	defaults := &propsTestModuleDefaults{}
	module := propsTestModule{}
	defaults.AddProperties(&module.props, &module.otherProps)
	InitDefaultsModule(defaults)
	return defaults
}

func (p *propsTestModule) GenerateAndroidBuildActions(ctx ModuleContext) {
	str := "abc"
	p.otherProps.F = &str
}

func TestUsedProperties(t *testing.T) {
	testCases := []struct {
		desc          string
		bp            string
		expectedProps []propInfo
	}{
		{
			desc: "only name",
			bp: `test {
			name: "foo",
		}
	`,
			expectedProps: []propInfo{
				propInfo{"Name", "string"},
			},
		},
		{
			desc: "some props",
			bp: `test {
			name: "foo",
			a: "abc",
			b: true,
			d: 123,
		}
	`,
			expectedProps: []propInfo{
				propInfo{"A", "string"},
				propInfo{"B", "bool"},
				propInfo{"D", "int64"},
				propInfo{"Name", "string"},
			},
		},
		{
			desc: "unused non-pointer prop",
			bp: `test {
			name: "foo",
			b: true,
			d: 123,
		}
	`,
			expectedProps: []propInfo{
				// for non-pointer cannot distinguish between unused and intentionally set to empty
				propInfo{"A", "string"},
				propInfo{"B", "bool"},
				propInfo{"D", "int64"},
				propInfo{"Name", "string"},
			},
		},
		{
			desc: "nested props",
			bp: `test {
			name: "foo",
			nested: {
				e: "abc",
			}
		}
	`,
			expectedProps: []propInfo{
				propInfo{"Nested.E", "string"},
				propInfo{"Name", "string"},
			},
		},
		{
			desc: "arch props",
			bp: `test {
			name: "foo",
			arch: {
				x86_64: {
					a: "abc",
				},
			}
		}
	`,
			expectedProps: []propInfo{
				propInfo{"Name", "string"},
				propInfo{"Arch.X86_64.A", "string"},
			},
		},
		{
			desc: "embedded props",
			bp: `test {
			name: "foo",
			embedded_prop: "a",
		}
	`,
			expectedProps: []propInfo{
				propInfo{"Embedded_prop", "string"},
				propInfo{"Name", "string"},
			},
		},
		{
			desc: "defaults",
			bp: `
test_defaults {
	name: "foo_defaults",
	a: "a",
	b: true,
	embedded_prop:"a",
	arch: {
		x86_64: {
			a: "a",
		},
	},
}
test {
	name: "foo",
	defaults: ["foo_defaults"],
	c: ["a"],
	nested: {
		e: "d",
	},
	target: {
		linux: {
			a: "a",
		},
	},
}
	`,
			expectedProps: []propInfo{
				propInfo{"A", "string"},
				propInfo{"B", "bool"},
				propInfo{"C", "string slice"},
				propInfo{"Embedded_prop", "string"},
				propInfo{"Nested.E", "string"},
				propInfo{"Name", "string"},
				propInfo{"Arch.X86_64.A", "string"},
				propInfo{"Target.Linux.A", "string"},
				propInfo{"Defaults", "string slice"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := GroupFixturePreparers(
				PrepareForTestWithAllowMissingDependencies,
				PrepareForTestWithDefaults,
				FixtureRegisterWithContext(func(ctx RegistrationContext) {
					ctx.RegisterModuleType("test", propsTestModuleFactory)
					ctx.RegisterModuleType("test_defaults", propsTestModuleDefaultsFactory)
				}),
				FixtureWithRootAndroidBp(tc.bp),
			).RunTest(t)

			foo := result.ModuleForTests("foo", "").Module().base()

			AssertDeepEquals(t, "foo ", tc.expectedProps, foo.propertiesWithValues())

		})
	}
}
