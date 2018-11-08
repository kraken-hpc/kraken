# Writing Kraken Modules

For ease of documentation, it will be assumed that the desired modulename is Dummy.

## Required Variables
### Word for Word
`var _ lib.Module = (*Dummy)(nil)`

`var _ lib.ModuleSelfService = (*_modulename_)(nil)`

`var _ lib.ModuleWithConfig = (*_modulename_)(nil)`

`var _ lib.ModuleWithDiscovery = (*_modulename_)(nil)`

`var _ lib.ModuleWithMutations = (*_modulename_)(nil)`

`var mutations = make(map[string]lib.StateMutation)`

`var discovers = make(map[string]map[string]reflect.Value)`

### "Variable" Variables
**muts**
  - type: `map[string][2]string`
  - describes the possible mutations
  
**timeouts**
  - type: `map[string]int`
  - describe a timeout for each mutation in the mutation list
  - `len(timeouts) == len(muts)`
  
**Dummy**
  - type: `struct`
  - contains the objects and variables needed for that module
## Methods

### Required Methods
`func (d *Dummy) Entry()`
- The module entry point
- The module should already have an initialized API and an established config before `Entry()` is called

`func (d *Dummy) Stop()`
- Stop is responsible for making sure the module exits cleanly

`func (d *Dummy) Name()`
- Name needs to return a uniquely identifying string
- The URL to the module is a good name choice

### Optional Methods
`func (d *Dummy) UpdateConfig(cfg proto.Message)`
- UpdateConfig takes a new module configuration, and potentially adjusts to changes it contains.
- Any module that needs configuration (most) should have this defined.
- If it is defined, updates will occur when they are changed in the state.

`func (d *Dummy) SetDiscoveryChan(c chan<- lib.Event)`
- Simply sets the discover channel

`func (d *Dummy) SetMutationChan(c <-chan lib.Event)`
- Simply sets the mutation channel

`func (d *Dummy) Init(api lib.APIClient)`
- Init initializes the module. It will be called before the API is initialized or the config is in place.
- If this exists, it will be called when the module is instantiated.
- It will be called before the API is initialized or the config is in place.

`func (d *Dummy) NewConfig() proto.Message`
- May be used in Entry() to initialize the config?

`func init()`
- Module needs to get registered here
- Use the mutations and discovers 
