import React, { useCallback, useEffect, useState } from 'react';
import { useSelector } from 'react-redux';

import { RootState } from '../../../initialState';
import { fetchGameControlStatus, gameControlPost, GameControlStatus, GameLab } from './gamecontrolApi';
import './GameControl.css';

const REFRESH_MS = 10000;

const TeacherPanel: React.FC = () => {
    const name = useSelector((state: RootState) => state.dashboard.name);
    const [status, setStatus] = useState<GameControlStatus | null>(null);
    const [error, setError] = useState<string>('');
    const [showHosts, setShowHosts] = useState<boolean>(false);

    const refresh = useCallback(async () => {
        const data = await fetchGameControlStatus();
        if (data) {
            setStatus(data);
        }
    }, []);

    useEffect(() => {
        refresh();
        const timer = window.setInterval(refresh, REFRESH_MS);

        return () => window.clearInterval(timer);
    }, [refresh]);

    const act = async (path: string, body: object) => {
        const res = await gameControlPost(path, body);
        setError(res.ok ? '' : res.error || 'Error');
        refresh();
    };

    const lab: GameLab | undefined = status?.labs[0];
    const cutCount = lab ? lab.hosts.filter((h) => h.internet_blocked).length : 0;

    return (
        <div className="teacher-panel">
            <div className="teacher-panel__bar">
                <div className="teacher-panel__title">Panel del Profesor</div>
                <div>
                    {name && <span className="mr-3 text-muted">{name}</span>}
                    <a href="control/logout" className="btn btn-sm btn-outline-secondary">
                        Cerrar sesión
                    </a>
                </div>
            </div>

            <div className="container pt-5 pb-5">
                {!status && <div>Cargando...</div>}

                {status && !lab && (
                    <div className="alert alert-warning">
                        Tu cuenta no tiene un laboratorio asignado. Pide al administrador que lo configure.
                    </div>
                )}

                {error && <div className="alert alert-danger">{error}</div>}

                {lab && (
                    <div className="card">
                        <div className="card-header">
                            <h3 className="card-title">{lab.name}</h3>
                            <div className="card-options">
                                <span className={`badge ${cutCount > 0 ? 'badge-danger' : 'badge-success'}`}>
                                    {cutCount > 0 ? `Internet cortado en ${cutCount} equipos` : 'Internet activo'}
                                </span>
                            </div>
                        </div>
                        <div className="card-body">
                            <div className="teacher-panel__group">
                                <div className="teacher-panel__label">Internet</div>
                                <div className="teacher-panel__buttons">
                                    <button
                                        className="btn btn-danger btn-lg teacher-panel__btn"
                                        onClick={() => act('internet/toggle_all', { lab_id: lab.id, blocked: true })}>
                                        Cortar Internet
                                    </button>
                                    <button
                                        className="btn btn-success btn-lg teacher-panel__btn"
                                        onClick={() => act('internet/toggle_all', { lab_id: lab.id, blocked: false })}>
                                        Restaurar Internet
                                    </button>
                                </div>
                            </div>

                            <div className="teacher-panel__group">
                                <div className="teacher-panel__label">Juegos</div>
                                <div className="teacher-panel__buttons">
                                    <button
                                        className="btn btn-danger btn-lg teacher-panel__btn"
                                        onClick={() => act('toggle_all', { lab_id: lab.id, blocked: true })}>
                                        Bloquear Juegos
                                    </button>
                                    <button
                                        className="btn btn-success btn-lg teacher-panel__btn"
                                        onClick={() => act('toggle_all', { lab_id: lab.id, blocked: false })}>
                                        Permitir Juegos
                                    </button>
                                </div>
                            </div>

                            <button className="btn btn-link p-0" onClick={() => setShowHosts(!showHosts)}>
                                {showHosts ? 'Ocultar equipos' : 'Ver equipos individuales'}
                            </button>

                            {showHosts && (
                                <div className="table-responsive mt-3">
                                    <table className="table table-vcenter card-table">
                                        <thead>
                                            <tr>
                                                <th>Equipo</th>
                                                <th>Juegos</th>
                                                <th>Internet</th>
                                                <th className="text-right">Acción</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {lab.hosts.map((h) => (
                                                <tr key={h.ip}>
                                                    <td className="font-weight-bold">{h.host}</td>
                                                    <td>
                                                        <span
                                                            className={`badge ${
                                                                h.blocked ? 'badge-danger' : 'badge-success'
                                                            }`}>
                                                            {h.blocked ? 'Bloqueado' : 'Permitido'}
                                                        </span>
                                                    </td>
                                                    <td>
                                                        <span
                                                            className={`badge ${
                                                                h.internet_blocked ? 'badge-danger' : 'badge-success'
                                                            }`}>
                                                            {h.internet_blocked ? 'Cortado' : 'Con acceso'}
                                                        </span>
                                                    </td>
                                                    <td className="text-right">
                                                        <div className="gamecontrol-row-actions">
                                                            <button
                                                                className={`btn btn-sm gamecontrol-row-btn ${
                                                                    h.blocked ? 'btn-success' : 'btn-danger'
                                                                }`}
                                                                onClick={() =>
                                                                    act('update_host', { ip: h.ip, blocked: !h.blocked })
                                                                }>
                                                                {h.blocked ? 'Permitir Acceso' : 'Bloquear Acceso'}
                                                            </button>
                                                            <button
                                                                className={`btn btn-sm gamecontrol-row-btn ${
                                                                    h.internet_blocked ? 'btn-success' : 'btn-danger'
                                                                }`}
                                                                onClick={() =>
                                                                    act('internet/toggle_host', {
                                                                        ip: h.ip,
                                                                        blocked: !h.internet_blocked,
                                                                    })
                                                                }>
                                                                {h.internet_blocked
                                                                    ? 'Restaurar Internet'
                                                                    : 'Cortar Internet'}
                                                            </button>
                                                        </div>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default TeacherPanel;
