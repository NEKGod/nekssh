const storageKey='nekssh.desktop.portUsage.v1';
const modes=new Set(['frequent','recent','newest','port']);
const number=value=>Number.isFinite(value)&&value>=0?value:0;

export function createPortHistory(storage){
  let saved;
  try{saved=JSON.parse(storage?.getItem(storageKey)||'null')}catch{}
  const usage=new Map();
  for(const [key,value] of Object.entries(saved?.usage||{})){
    if(value&&typeof value==='object') usage.set(key,{opens:number(value.opens),lastOpened:number(value.lastOpened),addedAt:number(value.addedAt)});
  }
  let mode=modes.has(saved?.mode)?saved.mode:'recent';
  if(saved?.sortDefaultsVersion!==2&&mode==='frequent') mode='recent';
  const persist=()=>{try{storage?.setItem(storageKey,JSON.stringify({mode,sortDefaultsVersion:2,usage:Object.fromEntries(usage)}))}catch{}};
  persist();
  const key=(server,tunnel)=>JSON.stringify([server,tunnel.RemoteAddress]);
  function stats(server,tunnel,now=Date.now()){
    const id=key(server,tunnel);
    if(!usage.has(id)){
      usage.set(id,{opens:0,lastOpened:0,addedAt:now});persist();
    }
    return {...usage.get(id)};
  }
  return {
    get mode(){return mode},
    setMode(value){if(modes.has(value)){mode=value;persist()}},
    stats,
    opened(server,tunnel,now=Date.now()){
      const value=stats(server,tunnel,now);
      usage.set(key(server,tunnel),{...value,opens:value.opens+1,lastOpened:now});persist();
    },
    sort(server,tunnels){
      const items=tunnels.map(tunnel=>({tunnel,stats:stats(server,tunnel)}));
      const added=item=>number(item.tunnel.CreatedAt)||item.stats.addedAt;
      return items.sort((a,b)=>{
        const frequency=b.stats.opens-a.stats.opens;
        const recent=b.stats.lastOpened-a.stats.lastOpened;
        const newest=added(b)-added(a);
        const port=(a.tunnel.RemotePort||0)-(b.tunnel.RemotePort||0);
        const primary=mode==='frequent'?frequency||recent:mode==='recent'?recent||frequency:mode==='newest'?newest:port;
        return primary||port||a.tunnel.RemoteAddress.localeCompare(b.tunnel.RemoteAddress)||a.tunnel.LocalAddress.localeCompare(b.tunnel.LocalAddress);
      }).map(item=>item.tunnel);
    }
  };
}
