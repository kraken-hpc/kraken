(window.webpackJsonp=window.webpackJsonp||[]).push([[0],{16:function(e,t,a){e.exports=a(29)},21:function(e,t,a){},29:function(e,t,a){"use strict";a.r(t);var n=a(0),o=a.n(n),c=a(13),r=a.n(c),s=(a(21),a(5)),l=a(6),d=a(8),i=a(7),u=a(9),m={yellow:"#f2cf66",red:"#e74c3c",grey:"#a5a5a5",purple:"#6a51ba",green:"#89CA78",blue:"#509EE8",black:"#000000"},f="192.168.57.10:3141";function N(e){return"PHYS_ERROR"===e|"ERROR"===e|"POWER_CYCLE"===e?m.red:"PHYS_HANG"===e?m.purple:"POWER_OFF"===e?m.grey:"PHYS_UNKNOWN"===e|"UNKNOWN"===e?m.yellow:"INIT"===e?m.blue:"SYNC"===e|"POWER_ON"===e?m.green:m.yellow}function v(e){for(var t=atob(e),a="",n=0;n<t.length;n++){var o=t.charCodeAt(n).toString(16);a+=2===o.length?o:"0"+o}return a.toUpperCase()}function h(e){for(var t=atob(e),a="",n=0;n<t.length;n++){var o=t.charCodeAt(n).toString(10);a+=0===n?o:"."+o}return a}function g(e){var t=v(e);return t=t.replace(/(.{8})(.{4})(.{4})(.{4})(.{10})/,"$1-$2-$3-$4-$5")}function E(e){return fetch(e).then(function(e){return e.json()}).then(function(e){return e.nodes}).catch(function(e){return null})}function p(e,t){return fetch(e,{headers:{"Content-Type":"application/json"},method:"PUT",body:JSON.stringify(t)}).then(function(e){return e.json()}).then(function(e){return console.log("Success:",JSON.stringify(e))}).catch(function(e){return console.error("Error:",e)})}var _=a(30);function y(){return o.a.createElement("a",{className:"title-link",href:"/"},o.a.createElement("h1",{className:"banner__title"},"Kraken"))}var w=function(e){function t(e){var a;return Object(s.a)(this,t),(a=Object(d.a)(this,Object(i.a)(t).call(this,e))).state={dsc_url:"http://".concat(f,"/dsc/nodes"),cfg_url:"http://".concat(f,"/cfg/nodes"),refreshRate:300,nodes:[],unknown_count:0,init_count:0,sync_count:0,dsc_loaded:!1},a}return Object(u.a)(t,e),Object(l.a)(t,[{key:"componentDidMount",value:function(){var e=this;this.interval=window.setInterval(function(){e.nodeInfoUpdate(e.state.dsc_url)},this.state.refreshRate),this.initialNodeFetch(this.state.cfg_url),this.nodeInfoUpdate(this.state.dsc_url)}},{key:"initialNodeFetch",value:function(e){var t=this;this.setState({loading:!0}),E(e).then(function(e){null===e?t.setState({disconnected:!0}):(e.sort(function(e,t){if("undefined"===typeof e.nodename&&"undefined"===typeof t.nodename)return 0;if("undefined"===typeof e.nodename)return-1;if("undefined"===typeof t.nodename)return 1;var a=e.nodename.match(/(\d*)/g),n=t.nodename.match(/(\d*)/g),o=parseInt(a[1]),c=parseInt(a[3]),r=parseInt(n[1]),s=parseInt(n[3]);return o>r?(console.log(""),1):o<r?-1:c>s?1:c<s?-1:e.nodename.localeCompare(t.nodename)}),t.setState({loading:!1,disconnected:!1,nodes:e}))})}},{key:"nodeInfoUpdate",value:function(e){var t=this;this.setState({loading:!0}),E(e).then(function(e){if(null===e)t.setState({disconnected:!0});else{t.state.nodes.length!==e.length&&t.initialNodeFetch(t.state.cfg_url);for(var a=0,n=0,o=0,c=0;c<e.length;c++)for(var r=0;r<t.state.nodes.length;r++)if(t.state.nodes[r].id===e[c].id)switch(t.state.nodes[r].physState=e[c].physState,t.state.nodes[r].runState=e[c].runState,t.state.nodes[r].runState){case"UNKNOWN":a++;break;case"INIT":n++;break;case"SYNC":o++;break;default:a++}t.setState({loading:!1,disconnected:!1,unknown_count:a,init_count:n,sync_count:o,dsc_loaded:!0})}})}},{key:"render",value:function(){return o.a.createElement(o.a.Fragment,null,this.state.disconnected&&o.a.createElement("h2",{style:{textAlign:"center",fontFamily:"Arial",color:"maroon"}},"Disconnected From Kraken"),"undefined"===typeof this.state.nodes[0]||!1===this.state.dsc_loaded?o.a.createElement("h3",{style:{fontFamily:"Arial"}},"Loading..."):o.a.createElement("div",{className:"cluster"},this.state.nodes.map(function(e){return o.a.createElement(b,{data:e,key:e.id})})),o.a.createElement("div",{className:"counts"},"Unknown: ",this.state.unknown_count),o.a.createElement("div",{className:"counts"},"Init: ",this.state.init_count),o.a.createElement("div",{className:"counts"},"Sync: ",this.state.sync_count))}}]),t}(n.Component);function b(e){"undefined"===typeof e.data.physState&&(e.data.physState="UNKNOWN"),"undefined"===typeof e.data.runState&&(e.data.runState="UNKNOWN");var t=N(e.data.physState),a=N(e.data.runState),n=g(e.data.id),c="Name: ".concat(e.data.nodename,"\nUUID: ").concat(n,"\nPhysical State: ").concat(e.data.physState,"\nRun State: ").concat(e.data.runState);return o.a.createElement(_.a,{"data-popup":c,className:"square shadow animate compute",style:{borderTopColor:t,borderRightColor:a,borderBottomColor:a,borderLeftColor:t},to:"node/".concat(n)})}function S(){return o.a.createElement("div",{className:"legend"},o.a.createElement("div",{className:"legend_title"},"Legend"),o.a.createElement("div",{className:"row",id:"first_row"},o.a.createElement("div",{className:"value",id:"phys_text"},"Phys"),o.a.createElement("div",{className:"square",style:{borderTopColor:m.grey,borderRightColor:m.black,borderBottomColor:m.black,borderLeftColor:m.grey}}),o.a.createElement("div",{className:"value",id:"run_text"},"Run")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.yellow}}),o.a.createElement("div",{className:"value"},"State Unknown")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.grey}}),o.a.createElement("div",{className:"value"},"Power Off")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.blue}}),o.a.createElement("div",{className:"value"},"Initializing")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.green}}),o.a.createElement("div",{className:"value"},"Power On / Sync")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.purple}}),o.a.createElement("div",{className:"value"},"Hang")),o.a.createElement("div",{className:"row"},o.a.createElement("div",{className:"square",style:{borderColor:m.red}}),o.a.createElement("div",{className:"value"},"Error")))}var O=function(e){function t(){return Object(s.a)(this,t),Object(d.a)(this,Object(i.a)(t).apply(this,arguments))}return Object(u.a)(t,e),Object(l.a)(t,[{key:"render",value:function(){return o.a.createElement(o.a.Fragment,null,o.a.createElement(y,null),o.a.createElement("div",{className:"main-body"},o.a.createElement(S,null),o.a.createElement("div",{className:"node-area"},o.a.createElement(w,null))))}}]),t}(n.Component),k=a(11),C=a.n(k);function I(){return o.a.createElement("a",{className:"title-link",href:"/"},o.a.createElement("h1",{className:"banner__title"},"Kraken"))}function j(e){var t=N(e.dscNode.physState),a=N(e.dscNode.runState);return o.a.createElement("div",{className:"large_square",style:{borderTopColor:t,borderRightColor:a,borderBottomColor:a,borderLeftColor:t}})}function P(e){var t,a;if(t="undefined"===typeof e.dscNode.runState?"UNKNOWN":e.dscNode.runState,a="undefined"===typeof e.cfgNode.runState?"UNKNOWN":e.cfgNode.runState,console.log("props",e),console.log("node id",e.cfgNode.id),null!==e.cfgNode.id&&"undefined"!==typeof e.cfgNode.id)var n=g(e.cfgNode.id);else n="";if(null!==e.cfgNode.parentId&&"undefined"!==typeof e.cfgNode.parentId)var c=g(e.cfgNode.parentId);else c="";return o.a.createElement("div",{className:"node_detail"},o.a.createElement("h1",{className:"node_title"},e.cfgNode.nodename),o.a.createElement("div",{className:"non_bordered_detail"},o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Node ID:"),o.a.createElement("div",{className:"node_view_value"},n)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Parent ID:"),o.a.createElement("div",{className:"node_view_value"},c)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Physical State:"),o.a.createElement("div",{className:"node_view_value"},e.dscNode.physState," / ",e.cfgNode.physState)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Run State:"),o.a.createElement("div",{className:"node_view_value"},t," / ",a)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Architecture:"),o.a.createElement("div",{className:"node_view_value"},e.cfgNode.arch)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Platform:"),o.a.createElement("div",{className:"node_view_value"},e.cfgNode.platform))),o.a.createElement(U,{cfgNode:e.cfgNode}))}function U(e){if(null!==e.cfgNode.extensions&&"undefined"!==typeof e.cfgNode.extensions)for(var t=0;t<e.cfgNode.extensions.length;t++)if("type.googleapis.com/proto.IPv4OverEthernet"===e.cfgNode.extensions[t]["@type"])var a=e.cfgNode.extensions[t];if(null!==a&&"undefined"!==typeof a)var n=a.ifaces[0].eth.iface,c=function(e){var t=v(e);return t=t.replace(/(.{2})(.{2})(.{2})(.{2})(.{2})(.{2})/,"$1:$2:$3:$4:$5:$6")}(a.ifaces[0].eth.mac),r=h(a.ifaces[0].ip.ip),s=h(a.ifaces[0].ip.subnet);else n="",c="",r="",s="";return o.a.createElement("div",{className:"bordered_detail"},o.a.createElement("div",{className:"info_title"},"Network"),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Interface:"),o.a.createElement("div",{className:"node_view_value"},n)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Mac Address:"),o.a.createElement("div",{className:"node_view_value"},c)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"IP Address:"),o.a.createElement("div",{className:"node_view_value"},r)),o.a.createElement("div",{className:"node_view_row"},o.a.createElement("div",{className:"node_view_key"},"Subnet Mask:"),o.a.createElement("div",{className:"node_view_value"},s)))}function x(e){return o.a.createElement("div",{className:"actions_area"},o.a.createElement("div",{className:"button red",onClick:function(){!function(e,t,a,n){t.runState="SYNC",t.physState="POWER_ON";for(var o=0;o<t.extensions.length;o++)"type.googleapis.com/proto.PXE"===t.extensions[o]["@type"]&&(t.extensions[o].state="INIT");p(n,t)}(e.dscNode,e.cfgNode,e.dscUrl,e.cfgUrl)}},"Power On"),o.a.createElement("div",{className:"button red",onClick:function(){!function(e,t,a,n){t.physState="POWER_OFF",t.runState="UNKNOWN";for(var o=0;o<t.extensions.length;o++)"type.googleapis.com/proto.PXE"===t.extensions[o]["@type"]&&(t.extensions[o].state="NONE");for(e.runState="UNKNOWN",e.physState="PHYS_HANG",o=0;o<e.extensions.length;o++)"type.googleapis.com/proto.PXE"===e.extensions[o]["@type"]&&(e.extensions[o].state="NONE");p(a,e),p(n,t)}(e.dscNode,e.cfgNode,e.dscUrl,e.cfgUrl)}},"Power Off"))}n.Component;var F=function(e){function t(e){var a;return Object(s.a)(this,t),(a=Object(d.a)(this,Object(i.a)(t).call(this,e))).state={dsc_url:"http://".concat(f,"/dsc/node/").concat(e.match.params.uuid),cfg_url:"http://".concat(f,"/cfg/node/").concat(e.match.params.uuid),dot_url:"http://".concat(f,"/graph/node/").concat(e.match.params.uuid,"/dot"),refreshRate:1e3,dscNode:{},cfgNode:{},dotString:"",disconnected:!0},a}return Object(u.a)(t,e),Object(l.a)(t,[{key:"componentDidMount",value:function(){var e=this;this.interval=window.setInterval(function(){e.nodeInfoFetch(e.state.dsc_url,"dsc")},this.state.refreshRate),this.nodeInfoFetch(this.state.cfg_url,"cfg"),this.textFetch(this.state.dot_url),this.nodeInfoFetch(this.state.dsc_url,"dsc")}},{key:"nodeInfoFetch",value:function(e,t){var a=this;this.setState({loading:!0}),function(e){return fetch(e).then(function(e){return e.json()}).catch(function(e){return console.warn(e),null})}(e).then(function(e){null===e&&a.setState({disconnected:!0}),"dsc"===t?a.setState({loading:!1,disconnected:!1,dscNode:e}):"cfg"===t?a.setState({loading:!1,disconnected:!1,cfgNode:e}):"dot"===t&&(console.log("response: ",e),a.setState({loading:!1,disconnected:!1,dotString:e}))})}},{key:"textFetch",value:function(e){var t=this;this.setState({loading:!0}),function(e){return fetch(e).then(function(e){return e.text()}).catch(function(e){return console.warn(e),null})}(e).then(function(e){console.log("response: ",e),t.setState({dotString:e})})}},{key:"render",value:function(){return console.log("dsc:",this.state.dscNode),console.log("cfg:",this.state.cfgNode),o.a.createElement(o.a.Fragment,null,o.a.createElement(I,null),this.state.disconnected||0===Object.keys(this.state.dscNode).length?o.a.createElement("h3",{style:{fontFamily:"Arial"}},"Loading..."):o.a.createElement("div",null,o.a.createElement("div",null,o.a.createElement(j,{dscNode:this.state.dscNode,cfgNode:this.state.cfgNode}),o.a.createElement(P,{dscNode:this.state.dscNode,cfgNode:this.state.cfgNode}),o.a.createElement(x,{dscNode:this.state.dscNode,cfgNode:this.state.cfgNode,dscUrl:this.state.dsc_url,cfgUrl:this.state.cfg_url}))))}}]),t}(n.Component),R=a(31),W=a(32),A=function(e){function t(){return Object(s.a)(this,t),Object(d.a)(this,Object(i.a)(t).apply(this,arguments))}return Object(u.a)(t,e),Object(l.a)(t,[{key:"render",value:function(){return o.a.createElement(R.a,null,o.a.createElement(o.a.Fragment,null,o.a.createElement(W.a,{exact:!0,path:"/",component:O}),o.a.createElement(W.a,{path:"/node/:uuid",component:F})))}}]),t}(n.Component);r.a.render(o.a.createElement(A,null),document.getElementById("root"))}},[[16,2,1]]]);
//# sourceMappingURL=main.7e9dfdbd.chunk.js.map