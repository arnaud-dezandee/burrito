import React from 'react';

import Button from '@/components/core/Button';

export interface ConfirmationModalProps {
  variant?: 'light' | 'dark';
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel?: () => void;
}

const ConfirmationModal: React.FC<ConfirmationModalProps> = ({
  variant = 'light',
  isOpen,
  onOpenChange,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  onConfirm,
  onCancel
}) => {
  const handleConfirm = () => {
    onConfirm();
    onOpenChange(false);
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    }
    onOpenChange(false);
  };

  const styles = {
    overlay: {
      light: 'bg-nuances-black/50',
      dark: 'bg-nuances-black/70'
    },
    modal: {
      light: 'bg-nuances-white text-nuances-black',
      dark: 'bg-nuances-400 text-nuances-50'
    },
    title: {
      light: 'text-nuances-black',
      dark: 'text-nuances-50'
    },
    message: {
      light: 'text-primary-600',
      dark: 'text-nuances-300'
    }
  };

  return (
    <>
      {isOpen && (
        <div
          className={`
            fixed
            inset-0
            z-50
            flex
            items-center
            justify-center
            ${styles.overlay[variant]}
          `}
          onClick={handleCancel}
        >
          <div
            className={`
              flex
              flex-col
              gap-8
              p-8
              rounded-2xl
              shadow-2xl
              max-w-lg
              w-full
              mx-4
              ${styles.modal[variant]}
            `}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex flex-col gap-6">
              <div className="flex flex-col gap-3">
                <h2
                  className={`
                    text-2xl
                    font-black
                    leading-7
                    ${styles.title[variant]}
                  `}
                >
                  {title}
                </h2>
                <div className="w-full h-px bg-nuances-200" />
              </div>
              <div className="flex flex-col gap-4">
                <p
                  className={`
                    text-base
                    font-normal
                    leading-6
                    ${styles.message[variant]}
                  `}
                >
                  {message}
                </p>
              </div>
            </div>
            <div className="flex gap-4 justify-end pt-2">
              <Button
                theme={variant}
                variant="secondary"
                onClick={handleCancel}
                className="min-w-24"
              >
                {cancelText}
              </Button>
              <Button
                theme={variant}
                variant="primary"
                onClick={handleConfirm}
                className="bg-status-error-default hover:bg-status-error-hover text-nuances-white min-w-24"
              >
                {confirmText}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default ConfirmationModal;
