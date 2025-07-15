import React from 'react';

import SyncIcon from '@/assets/icons/SyncIcon';

export interface RunningProps {
  action?: string;
}

const Running: React.FC<RunningProps> = ({ action }) => {
  const getDisplayText = () => {
    if (!action) return 'Running';

    switch (action.toLowerCase()) {
      case 'plan':
        return 'Planning';
      case 'apply':
        return 'Applying';
      default:
        return 'Running';
    }
  };

  return (
    <div className={`flex items-center gap-2 text-blue-500 fill-blue-500`}>
      <span className="text-sm font-semibold">{getDisplayText()}</span>
      <SyncIcon className="animate-spin-slow" height={16} width={16} />
    </div>
  );
};

export default Running;
