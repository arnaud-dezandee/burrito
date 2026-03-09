export type Stacks = {
  results: Stack[];
};

export type Stack = {
  uid: string;
  namespace: string;
  name: string;
  state: StackState;
  repository: string;
  branch: string;
  path: string;
  runCount: number;
  lastRunAt: string;
  lastRun: Run;
  latestRuns: Run[];
  lastResult: string;
  isRunning: boolean;
  manualSyncStatus: ManualSyncStatus;
  units: StackUnit[];
};

export type StackUnit = {
  id: string;
  path: string;
  state: string;
  lastAction: string;
  lastRunAt: string;
  lastResult: string;
  hasValidPlan: boolean;
  lastPlannedRevision: string;
  lastAppliedRevision: string;
  isRunning: boolean;
};

export type StackState = 'success' | 'warning' | 'error' | 'disabled';
export type ManualSyncStatus = 'none' | 'annotated' | 'pending';

export type Run = {
  id: string;
  commit: string;
  date: string;
  action: string;
};
