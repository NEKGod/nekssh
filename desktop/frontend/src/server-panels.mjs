// Requests share one SSH connection but use independent channels and render as ready.
export function createServerPanelLoader(fetchers){
  const flights=new Map();
  let revision=0;
  return {
    stop(){revision++},
    load(session,handlers){
      const current=++revision;
      return Promise.allSettled(Object.entries(fetchers).map(([name,fetch])=>{
        const key=JSON.stringify([session,name]);
        let request=flights.get(key);
        if(!request){
          request=Promise.resolve().then(()=>fetch(session));
          flights.set(key,request);
          const clear=()=>{if(flights.get(key)===request) flights.delete(key)};
          request.then(clear,clear);
        }
        return request.then(value=>{
          if(current===revision) handlers[name]?.(value);
        },error=>{
          if(current===revision) handlers.error?.(name,error);
        });
      }));
    }
  };
}
