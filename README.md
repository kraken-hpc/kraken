# Kraken

## Reorganization notice
***Please excuse our dust.  Kraken has been subdivided into different repos.  This repo now only contains core components for the kraken framework; most modules and extensions have been moved elsewhere.***

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

## I want to get involved...

Excellent!  It's our intention to make Kraken a community developed project.  To get started, you can:

1) contact us;
   Kraken has a Slack instance.  You can get an invite here: [slack.kraken-hpc.io](http://slack.kraken-hpc.io)
2) take a look at any posted issues;
3) post new issues;
4) create pull requests.

Enjoy!
