import React, { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';

import Card from '../../ui/Card';
import apiClient from '../../../api/Api';
import { addErrorToast, addSuccessToast } from '../../../actions/toasts';

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
            dispatch(addSuccessToast(t('config_profile_import_success')));
        } catch (error) {
            dispatch(addErrorToast({ error }));
        } finally {
            setImporting(false);
        }
    };

    return (
        <Card title={t('config_profile_title')} subtitle={t('config_profile_desc')} bodyType="card-body">
            <div className="form__group">
                <a
                    href={apiClient.getConfigProfileExportUrl()}
                    className="btn btn-outline-primary btn-standard mr-2"
                    download="adguard-escolar-profile.yaml">
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

            <div className="text-muted mt-2">{t('config_profile_import_hint')}</div>
        </Card>
    );
};
