import { merge } from 'lodash';
import { Apply, CortezaID, ISO8601Date, NoID } from '../../cast'
import { IsOf } from '../../guards'

interface Meta {
  name: string;
  description: string;
  visual: object;
  subWorkflow: boolean;
}

interface PartialWorkflow extends Partial<Omit<Workflow, 'meta' | 'createdAt' | 'updatedAt' | 'deletedAt' | 'suspendedAt'>> {
  meta?: Partial<Meta>;
  createdAt?: string|number|Date;
  updatedAt?: string|number|Date;
  deletedAt?: string|number|Date;
}

export class Workflow {
  public workflowID = NoID
  public handle = ''
  public runAs = NoID
  public enabled = true
  public labels = {}

  public paths = []
  public steps = []

  public meta: Meta = {
    name: '',
    description: '',
    visual: {},
    subWorkflow: false,
  }

  public ownedBy = NoID;
  public createdBy = NoID;
  public createdAt?: Date = undefined
  public updatedAt?: Date = undefined
  public deletedAt?: Date = undefined

  public canDeleteWorkflow = false
  public canExecuteWorkflow = false
  public canGrant = false
  public canManageWorkflowSessions = false
  public canManageWorkflowTriggers = false
  public canUndeleteWorkflow = false
  public canUpdateWorkflow = false

  constructor (w?: PartialWorkflow) {
    this.apply(w)
  }

  apply (w?: PartialWorkflow): void {
    Apply(this, w, CortezaID, 'workflowID')
    Apply(this, w, String, 'handle')

    Apply(this, w, Boolean, 'enabled', 'canDeleteWorkflow', 'canExecuteWorkflow', 'canGrant', 'canManageWorkflowSessions', 'canManageWorkflowTriggers', 'canUndeleteWorkflow', 'canUpdateWorkflow')

    Apply(this, w, ISO8601Date, 'createdAt', 'updatedAt', 'deletedAt')
    Apply(this, w, CortezaID, 'runAs', 'ownedBy', 'createdBy')

    if (w?.paths) {
      this.paths = w.paths
    }

    if (w?.steps) {
      this.steps = w.steps
    }

    if (IsOf(w, 'meta')) {
      this.meta = merge(this.meta, w.meta)
    }

    if (w?.labels) {
      this.labels = w.labels
    }
  }

  /**
   * Returns resource ID
   */
  get resourceID (): string {
    return `${this.resourceType}:${this.workflowID}`
  }

  /**
   * Resource type
   */
  get resourceType (): string {
    return 'automation:workflow'
  }
}
