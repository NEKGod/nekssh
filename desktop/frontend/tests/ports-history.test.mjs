import test from 'node:test';
import assert from 'node:assert/strict';
import {createPortHistory} from '../src/ports-history.mjs';

const tunnel=(port,CreatedAt=1)=>({ID:`local:${port}`,RemoteAddress:`127.0.0.1:${port}`,LocalAddress:`127.0.0.1:${port}`,RemotePort:port,CreatedAt});
function memoryStorage(){const data=new Map();return {getItem:key=>data.get(key),setItem:(key,value)=>data.set(key,value)}}

test('frequent browser opens persist across reload and changed local ports',()=>{
  const storage=memoryStorage(),history=createPortHistory(storage);
  history.setMode('frequent');
  const a=tunnel(80),b=tunnel(8888);
  history.opened('server',b,10);history.opened('server',b,20);history.opened('server',a,30);
  assert.deepEqual(history.sort('server',[a,b]),[b,a]);
  const restored=createPortHistory(storage),reconnected={...b,ID:'local:50000',LocalAddress:'127.0.0.1:50000'};
  assert.equal(restored.stats('server',reconnected).opens,2);
  assert.deepEqual(restored.sort('server',[a,reconnected]),[reconnected,a]);
  assert.equal(restored.stats('other-server',b).opens,0);
});

test('recent, newest and numeric port ordering work without mutating input',()=>{
  const storage=memoryStorage(),history=createPortHistory(storage);
  const a=tunnel(80,200),b=tunnel(8888,100),input=[a,b];
  history.opened('s',b,10);history.opened('s',a,20);
  history.setMode('recent');assert.deepEqual(history.sort('s',input),[a,b]);
  history.setMode('newest');assert.deepEqual(history.sort('s',[b,a]),[a,b]);
  assert.equal(createPortHistory(storage).mode,'newest');
  history.setMode('port');assert.deepEqual(history.sort('s',[b,a]),[a,b]);
  assert.deepEqual(input,[a,b]);
});

test('unavailable or corrupt storage does not break usage tracking',()=>{
  for(const storage of [undefined,{getItem(){throw Error()},setItem(){throw Error()}},{getItem:()=>'{invalid',setItem(){}}]){
    const history=createPortHistory(storage),a=tunnel(8888);
    history.opened('s',a,10);
    assert.equal(history.stats('s',a).opens,1);
    history.setMode('invalid');assert.equal(history.mode,'recent');
  }
});

test('recent clicks are the default and migrate the old default without losing usage',()=>{
  const storage=memoryStorage(),a=tunnel(80),b=tunnel(8888);
  const id=JSON.stringify(['s',b.RemoteAddress]);
  storage.setItem('nekssh.desktop.portUsage.v1',JSON.stringify({mode:'frequent',usage:{[id]:{opens:5,lastOpened:10,addedAt:1}}}));
  const history=createPortHistory(storage);
  assert.equal(history.mode,'recent');
  assert.equal(history.stats('s',b).opens,5);
  history.opened('s',a,20);
  assert.deepEqual(history.sort('s',[b,a]),[a,b]);
  assert.equal(createPortHistory(storage).mode,'recent');
  history.setMode('frequent');
  assert.equal(createPortHistory(storage).mode,'frequent');
  assert.equal(createPortHistory(memoryStorage()).mode,'recent');
});
