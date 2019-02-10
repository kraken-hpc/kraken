# NEW!  Want to give it a spin?

Look under `examples/vbox` for instructions to deploy a virtual Kraken cluster with a single command.  See the [README](examples/vbox/README.md).

# What is Kraken?

Kraken is a distributed state engine that can maintain state across a large set of computers.  It was designed to provide full-lifecycle maintenance of HPC compute clusters, from cold boot to ongoing system state maintenance and automation.  

Kraken was designed for HPC use-cases but may be useful anywhere distributed automation is needed. Kraken is designed to be highly modular and should be able to adapt to many situations.

# What do you mean by "state engine", and how does it work?

Kraken maintains a copy of the desired state (called "Configuration" state, abr "Cfg").  It also is able--through specialized modules--to discover bits of current real state ("Discoverable" state, abr. "Dsc").  Kraken modules provide a list of state mutations that they can perform, e.g. for `PhysState: POWER_OFF` to `PhysState: POWER_ON`.  These mutations are used to generate a directed graph.  Any time a difference is detected between Configuration (intended) state and Discoverable (actual) state, Kraken computes a path of mutations to converge on the Configuration state.

# What do you mean by "distributed state engine", and how does *that* work?

Kraken distributes the state across potentially thousands of individual physical nodes.  It maintains synchronization through a one-way state-update protocol, that is reminiscent of routing protocols like OSPF.  It can (or will when fully implemented) create trees for multi-level state synchronization, and each level of the tree can provide a full suite of Kraken controlled microservices.  State synchronization in Kraken follows the "eventual consistency" model; we never guarantee that the entire distributed state is consistent, but can provide conditional guaranties that it will converge to consistency.

# How do I learn more?

Kraken is brand new and in very active development.  As part of the development efforts of Kraken, we will be updating the repository with more and more documentation, ranging from implementation guides to application architecture and module API guides.

# Notes on this version of Kraken

Kraken is still a fledgling sea-monster.  We don't yet consider it ready for production but hope to get it there very soon.  If you try to deploy in production, it will likely drink your coffee/beer/tea.

Check back soon for more documentation, utilities and demonstrations.

# I want to get involved...

Excellent!  It's our intention to make Kraken a community developed project.  To get started, you can:

1) contact us;
2) take a look at any posted issues;
3) post new issues;
4) create pull requests.
