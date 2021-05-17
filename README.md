# Kraken

## Reorganization notice
***The Kraken project has been subdivided into different repos.  This repo now only contains core components for the kraken framework; most modules and extensions have been moved elsewhere.***

- If you're trying to provision clusters, you probably want [kraken-layercake](https://github.com/kraken-hpc/kraken-layercake)
- If you're looking for thermal control or Raspberry Pi modules/extensions, try [kraken-legacy](https://github.com/kraken-hpc/kraken-legacy)
- If you're looking for ansible roles for kraken, you probably want [kraken-layercake-ansible](https://github.com/kraken-hpc/kraken-layercake-ansible)

## What is Kraken?

Kraken is a distributed state engine *framework* for building tools that can maintain and manipulate state across a large set of computers.  It was designed to provide full-lifecycle maintenance of HPC compute clusters, from cold boot to ongoing system state maintenance and automation (see: [kraken-layercake](https://github.com/kraken-hpc/kraken-layercake) for this implementation).  

Kraken was designed for HPC use-cases but may be useful anywhere distributed automation is needed. Kraken is designed to be highly modular and should be able to adapt to many situations.

### Kraken is modular

Kraken on its own is only a framework.  When combined with modules, kraken can do things.  Kraken modules can discover real state values in a system an communicate them.  Kraken modules can declare that they know how to "mutate" the system to different states.  Kraken modules can also do things like run tiny services the system needs.

### Kraken state is extensible

Kraken starts with a very simple state-definition per node.  Through *extensions* kraken can define new kinds of state information.  You can loosely think of extensions as something like database schemas.

## What do you mean by "state engine", and how does it work?

Kraken maintains a copy of the desired state (called "Configuration" state, abr "Cfg").  It also is able--through specialized modules--to discover bits of current real state ("Discoverable" state, abr. "Dsc").  Kraken modules provide a list of state mutations that they can perform, e.g. for `PhysState: POWER_OFF` to `PhysState: POWER_ON`.  These mutations are used to generate a directed graph.  At any time, if a difference is detected between Configuration (intended) state and Discoverable (actual) state, Kraken computes a path of mutations to converge on the Configuration state.

## What do you mean by "distributed state engine", and how does *that* work?

Kraken distributes the state across potentially thousands of individual physical nodes.  It maintains synchronization through a one-way state-update protocol that is reminiscent of routing protocols like OSPF.  State synchronization in Kraken follows the "eventual consistency" model; we never guarantee that the entire distributed state is consistent, but can provide conditional guaranties that it will converge to consistency.

## How do I learn more?

Kraken is in very active development.  As part of the development efforts of Kraken, we will be updating the repository with more and more documentation, ranging from implementation guides to application architecture and module API guides.  There are also a number of talks, papers, and presentations out there about Kraken and its related projects.

## Notes on this version of Kraken

Kraken is still a fledgling sea-monster, but it has show itself capable of some pretty powerful things.  The [kraken-layercake](https://github.com/kraken-hpc/kraken-layercake) project is our reference project for kraken capabilities.  It can boot and maintain large scale compute clusters, providing unique capalities like:
- Stateful rolling updates of images in microseconds
- Self-healing capabilities at all layers of the stack
- Active feedback and monitoring of system state through tools like [kraken-dashboard](https://github.com/kraken-hpc/kraken-dashboard) and [krakenctl](https://github.com/kraken-hpc/krakenctl)

Check back soon for more documentation, utilities, and demonstrations.

## Generating a Kraken-based app

A Kraken-based app consists of three core componenets (and maybe more):

1. An application entry point, i.e. where a `main()` function lives.
2. A set of extensions that specify extra state variables for nodes.
3. A set of modules that define mutations and discover states.

The `kraken` command can be used to generate source code for these components.

First, get `kraken` with:

```shell
go get -u github.com/kraken-hpc/kraken
```

You'll also want to make sure `$GOPATH/bin` is in your executuion path, e.g. `export PATH=$PATH:$GOPATH/bin` .

### Generating an entry-point

You need an app definition to create an app.  This contains metadata about the app (like it's name and version) as well as references for what extensions and modules to include.

Here's an example for an app called `tester`:

```yaml
name: tester
version: "v0.1.1"
extensions:
  - "github.com/kraken-hpc/kraken/extensions/ipv4"
modules:
  - "github.com/kraken-hpc/kraken/modules/restapi"
  - "github.com/kraken-hpc/kraken/modules/websocket"
```

The canonical structure is to place this file in the directory where the entrypoint code should live, and name it `kraken.yaml`, but the naming is optional.  Assuming this convention:

```bash
$ mkdir tester
$ cd tester
<create kraken.yaml>
$ kraken app generate
INFO[0000] app "tester" generated at "."      
```

Now you can build your application:

```bash
$ go build .
$ ./tester -version
tester version: v0.1.1
this kraken is built with extensions: 
        type.googleapis.com/IPv4.IPv4OverEthernet
this kraken is built with modules: 
        github.com/kraken-hpc/kraken/modules/restapi
        github.com/kraken-hpc/kraken/modules/websocket
```

### Generating a module

You also need a definition to create a module using the `kraken` command.  The default name for this definition is `module.yaml` in the module directory.  Canonically, we use `<project>/modules/<module>` for module directories.

Module defintions are a more complicated than app definitions.  They need to define all of the mutations and discoveries that a module can do.  Here's an annotated example:

```yaml
---
# The package_url is the Go-style path to the module.  This must be a full url.
package_url: "github.com/kraken-hpc/kraken/modules/test"
# If with_polling is true, the module will be generated with a polling loop.
# The timer for the polling loop uses a config, so with_config is implied.
with_polling: true
# If with_config is specified, a stub for a protobuf config will be generated.
with_config: true
# This list declares any URLs we descover.
# The state of our own service and any URLs used in `mutates` sections of mutations below are automatically added.
# In this case, we're letting Kraken know that we discover things about `/RunState`, even though it's not part
# of our mutations.
discoveries:
  - "/RunState"
# This section declares mutations.  The key can be anything, but it must be unique. Ideally, it should be something descriptive.
# In this example we declare two mutations, one discovers /PhysState when it's unknown, the other mutates power from OFF to ON.
mutations:
  "Discover":
    mutates:
      "/PhysState":
        from: "PHYS_UNKNOWN"
        to: "POWER_OFF"
    requires:
      "/Platoform": "test"
    timeout: "10s"
    fail_to:
      url: "/PhysState"
      value: "PHYS_ERROR"
  "PowerON":
    mutates:
      "/PhysState":
        from: "POWER_OFF"
        to: "POWER_ON"
    requires:
      "/Platform": "test"
    timeout: "10s"
    fail_to:
      url: "/PhysState"
      value: "PHYS_ERROR"
```

Once we have the definition in place, we can generate the module with:

```bash
$ mkdir -p modules/test
$ cd modules/test
<create module.yaml>
$ kraken module generate 
INFO[0000] module "test" generated at "."
```

If we selected `with_config: true`, we will need to generate the protobuf code from the provided `proto` file.  You can add some variables to `test.config.proto` first, then:

```bash
$ cd modules/test
$ go generate
```

This will create `test.config.pb.go` (note: you need `protoc` and `gogo-proto` installed).

At this point, the module can be built and run, though it won't really do anything.  It will add to the generated graph, so this is a good point to make sure the graph output of an app that links this module is sane.

Finally, you'll want to edit `test.go`.  Unlike `test.mod.go`, which shouldn't be edited by hand, `test.go` is a stub to get you started and you'll need to edit it.  You'll need to add real function hanlders for your mutations in `func Init()`.  If you made a polling loop you probably want to put some stuff in `func Poll()`.  In general, look for comments starting with `// TODO:` for areas where you might want to alter things.

If you change your module definition, you can update your module without overwriting `test.go`.

```bash
$ kraken module update
```

This will *only* update `test.mod.go`.  Note: you may need to make manual changes to make `test.go` match, e.g. if you changed your list of mutations.

### Generating an extension

Extensions are the least complicated to generate.  The definition file for an extension looks like:

```yaml
---
package_url: github.com/kraken-hpc/kraken/test
name: TestMessage
custom_types:
  - "MySpecialType"  
```

This will generate an extension that will be referenced as `Test.TestMessage`.  Note that we support generating multiple extensions in the same proto package using multiple definition files.  E.g., you could also have `Test.AnotherMessage` defined in another file.

The procedure is similar to the others.  The default file name for extensions is `extension.yaml`:

```bash
$ mkdir -p extensions/test
$ cd extensions/test
<create extension.yaml>
$ kraken extension generate 
INFO[0000] extension "Test.TestMessage" generated at "."  
```

## I want to get involved...

Excellent!  It's our intention to make Kraken a community developed project.  To get started, you can:

1) contact us;
   Kraken has a Slack instance.  You can get an invite here: [slack.kraken-hpc.io](http://slack.kraken-hpc.io)
2) take a look at any posted issues;
3) post new issues;
4) create pull requests.

Enjoy!
