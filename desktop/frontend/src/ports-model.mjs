export function portTargetKey(port){
  let address=port.Address;
  if(!address||address==='*'||address==='0.0.0.0') address='127.0.0.1';
  if(address==='::') address='::1';
  return `${address.includes(':')?'['+address+']':address}:${port.Port}`;
}

export function groupListeningPorts(ports){
  const groups=new Map();
  ports.forEach((port,index)=>{
    if(!groups.has(port.Port)) groups.set(port.Port,{Port:port.Port,Entries:[]});
    groups.get(port.Port).Entries.push({port,index});
  });
  return [...groups.values()].sort((a,b)=>a.Port-b.Port);
}

export function tcpPortChoices(group){
  const seen=new Set();
  return group.Entries.filter(({port})=>{
    if(port.Protocol!=='tcp') return false;
    const key=portTargetKey(port);
    if(seen.has(key)) return false;
    seen.add(key);return true;
  });
}
