import React, { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';

import Card from '../../ui/Card';
import apiClient from '../../../api/Api';
import { addErrorToast, addSuccessToast } from '../../../actions/toasts';

const RESTART_POLL_MS = 2000;
const RESTART_TIMEOUT_MS = 120000;

const sleep = (ms: number) =>
    new Promise((resolve) => {
        setTimeout(resolve, ms);
    });

/**
 * pollUntilUp resolves once the server answers again, or rejects once deadline
 * (a Date.now() value) has passed.
 */
const pollUntilUp = async (deadline: number): Promise<void> => {
    try {
        const res = await fetch('control/status', { cache: 'no-store' });
        if (res.ok) {
            return;
        }
    } catch {
        // Still restarting.
    }

    if (Date.now() > deadline) {
        throw new Error('timeout');
    }

    await sleep(RESTART_POLL_MS);

    await pollUntilUp(deadline);
};

/**
 * waitForRestart resolves once the server answers again after restarting, or
 * rejects after RESTART_TIMEOUT_MS.
 */
const waitForRestart = async () => {
    // Give the server time to go down before polling.
    await sleep(RESTART_POLL_MS * 2);

    await pollUntilUp(Date.now() + RESTART_TIMEOUT_MS);
};

/**
 * ConfigProfile lets an admin export the portable subset of this
 * installation's configuration (DNS behavior, blocklists, blocked services,
 * safe search, cache, query log and statistics retention) to a file, and
 * import one previously exported from another installation. Installation-
 * specific settings (bind address, users, TLS, DHCP, clients) are never
 * touched.
 */
export const ConfigProfile = () => {
    const { t } = useTranslation();
    const dispatch = useDispatch();

    const fileInputRef = useRef<HTMLInputElement>(null);
    const [importing, setImporting] = useState(false);
    const [restarting, setRestarting] = useState(false);

    const handleImportClick = () => {
        fileInputRef.current?.click();
    };

    const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        const input = fileInputRef.current;

        if (input) {
            input.value = '';
        }

        if (!file) {
            return;
        }

        setImporting(true);

        try {
            const profileText = await file.text();
            await apiClient.importConfigProfile(profileText);
        } catch (error) {
            dispatch(addErrorToast({ error }));
            setImporting(false);

            return;
        }

        dispatch(addSuccessToast(t('config_profile_import_success')));
        setRestarting(true);

        try {
            await waitForRestart();
            window.location.reload();
        } catch {
            setRestarting(false);
            setImporting(false);
            dispatch(addErrorToast({ error: t('config_profile_restart_timeout') }));
        }
    };

    return (
        <Card title={t('config_profile_title')} subtitle={t('config_profile_desc')} bodyType="card-body">
            <div className="form__group">
                <a
                    href={apiClient.getConfigProfileExportUrl()}
                    className="btn btn-outline-primary btn-standard mr-2"
                    download="escolarapp-shield-perfil.yaml">
                    {t('config_profile_export')}
                </a>

                <button
                    type="button"
                    className="btn btn-outline-primary btn-standard"
                    onClick={handleImportClick}
                    disabled={importing}>
                    {t('config_profile_import')}
                </button>

                <input
                    ref={fileInputRef}
                    type="file"
                    accept=".yaml,.yml"
                    className="d-none"
                    onChange={handleFileChange}
                    data-testid="config_profile_file_input"
                />
            </div>

            {restarting ? (
                <div className="alert alert-info mt-2 mb-0">{t('config_profile_restarting')}</div>
            ) : (
                <div className="text-muted mt-2">{t('config_profile_import_hint')}</div>
            )}
        </Card>
    );
};
