import test from 'node:test';
import assert from 'node:assert/strict';
import {validProjectName,projectTemplates,vscodeProjectURL} from '../src/project-model.mjs';
test('VS Code opens only the remote project folder through Listen',()=>{
  const url=new URL(vscodeProjectURL('0.0.0.0:8888','/srv/我的项目 #1?x','https'));
  assert.equal(url.origin,'https://127.0.0.1:8888');
  assert.deepEqual([...url.searchParams],[['folder','/srv/我的项目 #1?x']]);
  assert.equal(new URL(vscodeProjectURL('[::]:8888','/srv/app')).hostname,'[::1]');
  for(const address of ['user:pass@host:80','localhost:80/path','localhost:80?x=1','https://host:80'])assert.throws(()=>vscodeProjectURL(address,'/srv/app'));
  assert.throws(()=>vscodeProjectURL('localhost:80','relative'));
  assert.throws(()=>vscodeProjectURL('localhost:80','/srv/app','file'));
});
test('project names reject traversal and shell metacharacters',()=>{
  for(const name of ['../app','a/b','a;b','','.','..','a\n',String('a').repeat(65)])assert.equal(validProjectName(name),false);
  assert.equal(validProjectName('my-app_2'),true);
});
test('templates only describe relative project files',()=>{
  assert.equal(new Set(projectTemplates.map(t=>t.id)).size,4);
  for(const template of projectTemplates){for(const file of template.files){assert.ok(!file.includes('/')&&!file.includes('..'))}}
});
