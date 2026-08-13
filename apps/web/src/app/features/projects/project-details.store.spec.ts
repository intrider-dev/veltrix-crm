import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Project } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ProjectDetailsStore } from './project-details.store';

const project: Project = {
  id: 'project-1',
  name: 'Launch',
  description: 'Customer launch',
  status: 'active',
  visibility: 'restricted',
  capabilities: { canView: true, canComment: true, canEdit: true, canManage: true },
  version: 2,
  createdAt: '2026-07-22T00:00:00Z',
  updatedAt: '2026-07-22T00:00:00Z',
};

describe('ProjectDetailsStore', () => {
  it('loads one capability-scoped aggregate without unbounded collections', async () => {
    const api = {
      getProject: vi.fn().mockResolvedValue({ body: project, etag: '"2"' }),
      listProjectAssignments: vi.fn().mockResolvedValue({ items: [], version: project.version }),
      listActivities: vi.fn().mockResolvedValue([]),
      listMembers: vi.fn().mockResolvedValue([]),
      listDepartments: vi.fn().mockResolvedValue([]),
    };
    TestBed.configureTestingModule({
      providers: [
        ProjectDetailsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });

    const store = TestBed.inject(ProjectDetailsStore);
    await store.load(project.id);

    expect(store.project()).toEqual(project);
    expect(api.listActivities).toHaveBeenCalledWith('workspace-1', 'project', project.id);
  });

  it('replaces assignments with the complete deduplicated set', async () => {
    const assignment = {
      id: 'assignment-1',
      kind: 'responsible' as const,
      subjectType: 'user' as const,
      subjectId: 'user-1',
      displayName: 'Alex',
      createdAt: '2026-07-22T00:00:00Z',
    };
    const api = {
      replaceProjectAssignments: vi.fn().mockResolvedValue({ items: [assignment], version: 3 }),
    };
    TestBed.configureTestingModule({
      providers: [
        ProjectDetailsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ProjectDetailsStore);
    store.project.set(project);
    store.assignmentVersion.set(project.version);

    await store.addAssignment({
      kind: 'responsible',
      subjectType: 'user',
      subjectId: 'user-1',
    });

    expect(api.replaceProjectAssignments).toHaveBeenCalledWith(
      'workspace-1',
      project.id,
      project.version,
      [{ kind: 'responsible', subjectType: 'user', subjectId: 'user-1' }],
    );
    expect(store.assignments()).toEqual([assignment]);
    expect(store.project()?.version).toBe(3);
  });
});
