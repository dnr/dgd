
# dynamic go derivations

This is a build system for Go binaries in Nix using dynamic derivations. Dynamic
derivations are an experimental Nix feature that allows Nix to extend its build
plan as it's building, which allows for very fine-grained derivations, which we
can use for better build caching.

## design

Go's `go` command-line tool is the primary way of building Go programs. It's
pretty opaque, it wants to build a whole binary at once as a black box, and do
its own caching, which fights with Nix's sandboxing.

On the other hand, reimplementing all of `go`'s logic and keeping it up to date
seems impractical (that's what [Bazel][1] does, but probably no one else).

[1]: https://github.com/bazel-contrib/rules_go

How do we avoid reimplementing `go`, but break apart the build so we can package
it in multiple derivations?

The “trick”—you might say gross hack, and you'd be right—is to run `go build -n`
to output a script of what `go` will do, and then parse it and process it into
Nix derivations.

That's the core idea, the rest just wraps it up nicely, with a bunch of special
handling around third-party dependencies.

### handling dependencies

Similar to `buildGoModule`, DGD requires a `vendorHash` for third-party
dependencies. This lets it use a single FOD for dependencies. So when you add or
remove a dependency, you have to update `vendorHash`, which will download all
dependencies again.

But! DGD breaks apart the single FOD into a separate FOD per module, which are
the ones used in the build derivations. So adding/removing a dependency doesn't
require re-*building* any other dependencies.

Note that the dependency FOD has a different structure than `buildGoModule`, so
it takes a different `vendorHash`. In principle, it could be changed to use the
same structure, but it was annoying, maybe in the future.

### source packages

DGD also splits apart your main module into separate Nix store paths so only
changed packages need to be re-built.

### dynamic derivations vs IFD

For this purpose, dynamic derivations are mostly just a "better IFD" that
doesn't block evaluation. So, this can do IFD too! Most of the code is the same.

## advantages

- Fine-grained derivations and perfect Go build caching down to the package
  level, for both the main module and third-party dependencies.
- Should be relatively easy to adapt to new Go versions and features.

## limitations

- It's a crazy gross hack that's inherently kinda fragile.
- It's slower than a plain Go build. Something like 1.5×. All that sandboxing…
- Single `vendorHash` means re-downloading (but not re-building) all deps when
  one changes. This could be fixed with a command to generate a set of FODs like
  `gomod2nix`.
- Dynamic derivations are experimental and the interface may change.

## missing features and future work

- Most `buildGoModule` features aren't supported yet:
  - `ldflags`
  - `nativeBuildInputs` (but CGO works in general)
  - any other go command line flags
  - stripping and other post-processing
- The final binary is placed in the store as a single file, without `/bin/` structure.
- Building tests only.
- Running tests.

Most of these should be pretty easy to add. Send a PR!

## demo

```
# Run example builds with dynamic derivations:
nix-build -A testExamples --arg useDynDrv true

# Run example builds with IFD:
nix-build -A testExamples --arg useDynDrv false

# Run both modes in a VM (to avoid messing with your system Nix):
nix-build -A vmtest
```

## interface

Import DGD into your project somehow and then use either `buildWithDynDrv` or
`buildWithIFD`. They expose the same interface:

- `src`: (required) Path to your main module.
- `vendorHash`: (optional) Hash for third-party deps.
- `env`: (optional) Extra environment vars to pass to build (put `CGO_ENABLED` here if you want it).
- `subPackage` (optional): If you want to build a package that's not the module
  root, put its path here, relative to the module root, without a `./` prefix.
- `go`: (optional) Go version to use for the build.
- `pkgs`: (optional) Override nixpkgs.
- `innerNix` (optional) Override nix.

## references

- [Dynamic Derivations in the Nix manual](https://nix.dev/manual/nix/latest/development/experimental-features#xp-feature-dynamic-derivations)
- [RFC 92](https://github.com/NixOS/rfcs/blob/master/rfcs/0092-plan-dynamism.md)
- Farid Zakaria's excellent blog posts:
  - [An early look at Nix Dynamic Derivations](https://fzakaria.com/2025/03/10/an-early-look-at-nix-dynamic-derivations)
  - [Nix Dynamic Derivations: A practical application](https://fzakaria.com/2025/03/11/nix-dynamic-derivations-a-practical-application)
  - [Nix Dynamic Derivations: A lang2nix practicum](https://fzakaria.com/2025/03/12/nix-dynamic-derivations-a-lang2nix-practicum)


