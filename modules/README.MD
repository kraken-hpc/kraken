# What is a Kraken Module?
- A Kraken module is an add-on to the core Kraken program. 
- Modules speak to Kraken through discoveries and mutations. 
  - A discovery occurs when a module determines that a change has occurred. The module passes that change along to Kraken so that it can keep track of what's going on.
  - A mutation is a desired change which corresponds with an edge in the state mutation engine. Mutations are sent by Kraken to modules. The module is responsible for performing a task based on the received mutation.

# Writing a Kraken Module
For ease of documentation, it will be assumed that the module being worked on is `Ipmipower`. Since, we are making an `Ipmipower` module, we should start by creating an `Ipmipower` struct. That object will implement some number of the interfaces listed below:

# Interfaces
## Module
- Must be implemented
### Required Methods
- `func (p *Ipmipower) Name() string`
  - returns the name of the module

## ModuleSelfService
- Must be implemented
### Required Methods
- `func (p *Ipmipower) Entry()`
  - `Entry()` is the start point of the module
  - `Entry()` is where the main portion of the module code will reside
- `func (p *Ipmipower) Init(api lib.APIClient)`
  - `Init()` is the first function to run when the module starts
  - Initializes the member variables of the module struct
- `func (p *Ipmipower) Stop()`
  - `Stop()` ensures clean exit of the module

## ModuleWithConfig
- Should be implemented if the module is to allow any customizability
### Required Methods
- `func (*Ipmipower) NewConfig() proto.Message`
  - Returns the default configuration
- `func (p *Ipmipower) UpdateConfig(cfg proto.Message) (e error)`
  - Writes a new module configuration
- `func (p *Ipmipower) ConfigURL() string`
  - Returns the configuration URL
### Required Variables
- `var cfg *proto.IpmipowerConfig`
  - `cfg` stores the module configuration
  - `*proto.IpmipowerConfig` is defined in the ipmipower.proto file, which generates the ipmipower.pb.go file

## ModuleWithDiscovery
- Should be implemented if the module is to communicate discoveries to Kraken
### Required Methods
- `func (p *Ipmipower) SetDiscoveryChan(c chan<- lib.Event)`
  - Sets the channel on which discoveries will be sent
### Required Variables
- `var dchan chan<- lib.Event`
  - The channel through which discoveries are sent

## ModuleWithMutations
- Should be implemented if the module is to receive notifaction of mutations
### Required Methods
- `func (p *Ipmipower) SetMutationChan(c <-chan lib.Event)`
  - Sets the channel on which mutations will be received
### Required Variables
- `var mchan <-chan lib.Event`
  - The channel through which mutations are received

# Module Registration
- There are certain things that must be registered with Kraken in order for proper communication to occur
  - `core.Registry.RegisterModule(&Ipmipower)`
    - Required for every module
  - `core.Registry.RegisterServiceInstance(&Ipmipower, )`
    - Required for every module
  - `core.Registry.RegisterDiscoverable(&Ipmipower, discoverables)`
    - Required if the module implements the `ModuleWithDiscovery` interface
    - `discoverables` is of type `map[string]map[string]reflect.Value`
  - `core.Registry.RegisterMutations(&Ipmipower, mutations)`
    - Required if the module implements the `ModuleWithMutations` interface
    - `mutations` is of type `map[string]lib.StateMutation`

# The `MutationEvent` Object
- There are two `Node` member variables in the `MutationEvent` struct: `NodeCfg` and `NodeDsc`
  - `NodeCfg` contains complete information about the desired configuration of the node
  - `NodeDsc` contains incomplete information about the current configuration of the node

# Logging
- May be performed using `&Ipmitool.api.Logf(loggingLevel, errorMessage)`
  - `loggingLevel` is of type `level LoggerLevel`
  - `errorMessage` is of type `fmt string`

# How do I add my module to the run list?
- `modules.go` is responsible for loading all of the modules to be used with Kraken at the time of start 
- Add your module to the list in `module.go` and restart Kraken
- dynamic module loading will likely be added as a feature in the future

# Questions
- What goes in the config versus in the member variables and should configuration variables and member variables overlap?
- 