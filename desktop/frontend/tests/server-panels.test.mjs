import test from 'node:test';
import assert from 'node:assert/strict';
import {createServerPanelLoader} from '../src/server-panels.mjs';

function deferred(){
  let resolve,reject;
  const promise=new Promise((yes,no)=>{resolve=yes;reject=no});
  return {promise,resolve,reject};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));

test('starts all panels together and renders fast panels before slow CPU sampling',async()=>{
  const pending={stats:deferred(),processes:deferred(),partitions:deferred()};
  const started=[],rendered=[];
  const loader=createServerPanelLoader(Object.fromEntries(Object.entries(pending).map(([name,request])=>[name,()=>{started.push(name);return request.promise}])));
  const complete=loader.load('server-a',Object.fromEntries(Object.keys(pending).map(name=>[name,()=>rendered.push(name)])));
  await tick();
  assert.deepEqual(started,['stats','processes','partitions']);
  pending.processes.resolve([]);await tick();
  assert.deepEqual(rendered,['processes']);
  pending.partitions.resolve([]);await tick();
  assert.deepEqual(rendered,['processes','partitions']);
  pending.stats.resolve({});await complete;
  assert.deepEqual(rendered,['processes','partitions','stats']);
});

test('one failed panel does not suppress successful panels and can retry',async()=>{
  const results=[],errors=[];let attempts=0;
  const loader=createServerPanelLoader({stats:()=>({CPUPercent:10}),processes:()=>[],partitions:()=>{if(++attempts===1)throw Error('df failed');return []}});
  const handlers={stats:()=>results.push('stats'),processes:()=>results.push('processes'),partitions:()=>results.push('partitions'),error:name=>errors.push(name)};
  await loader.load('a',handlers);
  assert.deepEqual(results,['stats','processes']);assert.deepEqual(errors,['partitions']);
  await loader.load('a',handlers);
  assert.equal(attempts,2);assert(results.includes('partitions'));
});

test('switching servers prevents an old result from overwriting the active server',async()=>{
  const a=deferred(),b=deferred(),rendered=[];
  const loader=createServerPanelLoader({stats:id=>id==='a'?a.promise:b.promise});
  const first=loader.load('a',{stats:()=>rendered.push('a')});loader.stop();
  const second=loader.load('b',{stats:()=>rendered.push('b')});
  b.resolve({});await second;a.resolve({});await first;
  assert.deepEqual(rendered,['b']);
});

test('polling and switching away and back reuse in-flight work without stale rendering',async()=>{
  const request=deferred(),rendered=[];let calls=0;
  const loader=createServerPanelLoader({stats:()=>{calls++;return request.promise}});
  const first=loader.load('a',{stats:()=>rendered.push('old')});await tick();loader.stop();
  const second=loader.load('a',{stats:()=>rendered.push('active')});await tick();
  assert.equal(calls,1);request.resolve({});await Promise.all([first,second]);
  assert.deepEqual(rendered,['active']);
});

test('disconnect discards pending results',async()=>{
  const request=deferred(),rendered=[];
  const loader=createServerPanelLoader({stats:()=>request.promise});
  const complete=loader.load('a',{stats:()=>rendered.push('stats')});
  loader.stop();request.resolve({});await complete;
  assert.deepEqual(rendered,[]);
});
