import test from 'node:test';
import assert from 'node:assert/strict';
import {createSettingsStore,defaultSettings,normalizeSettings,settingsKey,terminalFontStack} from '../src/settings-model.mjs';
const storage=()=>{const values=new Map();return {getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,value)}};

test('settings persist, restore and do not mutate caller objects',()=>{
  const memory=storage(),store=createSettingsStore(memory),draft={...store.get(),fontSize:18,accent:'blue',statsInterval:30};
  store.save(draft);draft.fontSize=24;
  assert.equal(store.get().fontSize,18);assert.deepEqual(createSettingsStore(memory).get(),store.get());
  const copy=store.get();copy.debug=true;assert.equal(store.get().debug,false);
});
test('legacy sidebar and debug preferences migrate without deleting unrelated data',()=>{
  const memory=storage();memory.setItem('nekssh.desktop.sidebar.collapsed','1');memory.setItem('nekssh.desktop.debug','1');memory.setItem('nekssh.desktop.hosts','private connections');
  const store=createSettingsStore(memory);assert.equal(store.get().showSidebar,false);assert.equal(store.get().debug,true);
  store.save(defaultSettings);assert.equal(memory.getItem('nekssh.desktop.hosts'),'private connections');
});
test('invalid stored types, unsupported choices and terminal limits fall back safely',()=>{
  const invalid={fontSize:1e9,lineHeight:0,fontFamily:'bad font',scrollback:-1,accent:'red',density:'unknown',debug:'true',cursorBlink:0,showSidebar:null,portsInterval:0,statsInterval:1,cursorStyle:'unknown'};
  assert.deepEqual(normalizeSettings(invalid),defaultSettings);
  const memory=storage();memory.setItem(settingsKey,'{broken');assert.deepEqual(createSettingsStore(memory).get(),defaultSettings);
  assert.deepEqual(normalizeSettings(null),defaultSettings);
});
test('unavailable storage reports failure instead of losing settings silently',()=>{
  const store=createSettingsStore({getItem(){throw Error('read denied')},setItem(){throw Error('disk full')}});
  assert.deepEqual(store.get(),defaultSettings);assert.throws(()=>store.save({...defaultSettings,fontSize:20}),/disk full/);
  assert.deepEqual(store.get(),defaultSettings);assert.throws(()=>createSettingsStore().save(defaultSettings));
});
test('system monospace stays a generic font rather than a quoted family name',()=>{
  assert.equal(terminalFontStack('monospace'),'monospace');
  assert.equal(terminalFontStack('Cascadia Mono'),'"Cascadia Mono", Consolas, monospace');
});
