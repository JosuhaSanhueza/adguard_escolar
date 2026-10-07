import React, { useCallback, useEffect, useState } from 'react';
import PageTitle from '../../ui/PageTitle';
import Card from '../../ui/Card';
import LabsAdmin from './LabsAdmin';
import './GameControl.css';
import { fetchGameControlStatus, gameControlPost, GameControlStatus, GameHost, GameLab } from './gamecontrolApi';

const GameControl: React.FC = () => {
    const [status, setStatus] = useState<GameControlStatus | null>(null);
    const [loading, setLoading] = useState<boolean>(true);
    const [search, setSearch] = useState<string>('');
    const [error, setError] = useState<string>('');

    const refresh = useCallback(async () => {
        const data = await fetchGameControlStatus();
        if (data) {
            setStatus(data);
        }
        setLoading(false);
    }, []);

    useEffect(() => {
        refresh();
    }, [refresh]);

    const act = async (path: string, body: object) => {
        const res = await gameControlPost(path, body);
        setError(res.ok ? '' : res.error || 'Error');
        refresh();
    };

    const matches = (h: GameHost) =>
        h.host.toLowerCase().includes(search.toLowerCase()) || h.ip.includes(search);

    const renderLab = (lab: GameLab) => {
        const hosts = lab.hosts.filter(matches);

        return (
            <Card key={lab.id} title={`${lab.name} (${lab.range_start} - ${lab.range_end})`}>
                <div className="p-3">
                    <div className="mb-4">
                        <div className="gamecontrol-bulk-row mb-2">
                            <span className="font-weight-bold" style={{ minWidth: 70 }}>
                                Juegos:
                            </span>
                            <button
                                className="btn btn-danger btn-sm gamecontrol-bulk-btn"
                                onClick={() => act('toggle_all', { lab_id: lab.id, blocked: true })}>
                                Bloquear Juegos
                            </button>
                            <button
                                className="btn btn-success btn-sm gamecontrol-bulk-btn"
                                onClick={() => act('toggle_all', { lab_id: lab.id, blocked: false })}>
                                Desbloquear Juegos
                            </button>
                        </div>
                        <div className="gamecontrol-bulk-row">
                            <span className="font-weight-bold" style={{ minWidth: 70 }}>
                                Internet:
                            </span>
                            <button
                                className="btn btn-danger btn-sm gamecontrol-bulk-btn"
                                onClick={() => act('internet/toggle_all', { lab_id: lab.id, blocked: true })}>
                                Cortar Internet
                            </button>
                            <button
                                className="btn btn-success btn-sm gamecontrol-bulk-btn"
                                onClick={() => act('internet/toggle_all', { lab_id: lab.id, blocked: false })}>
                                Restaurar Internet
                            </button>
                        </div>
                    </div>

                    <div className="table-responsive">
                        <table className="table table-vcenter card-table">
                            <thead>
                                <tr>
                                    <th>Equipo / Host</th>
                                    <th>Dirección IP</th>
                                    <th>Estado de Acceso a Juegos</th>
                                    <th>Acceso a Internet</th>
                                    <th className="text-right">Acción</th>
                                </tr>
                            </thead>
                            <tbody>
                                {hosts.map((h) => (
                                    <tr key={h.ip}>
                                        <td className="font-weight-bold">{h.host}</td>
                                        <td>{h.ip}</td>
                                        <td>
                                            <span className={`badge ${h.blocked ? 'badge-danger' : 'badge-success'}`}>
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
                                                    onClick={() => act('update_host', { ip: h.ip, blocked: !h.blocked })}>
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
                                                    {h.internet_blocked ? 'Restaurar Internet' : 'Cortar Internet'}
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                                {hosts.length === 0 && (
                                    <tr>
                                        <td colSpan={5} className="text-center text-muted">
                                            No se encontraron equipos.
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>
            </Card>
        );
    };

    return (
        <div>
            <PageTitle title="GameControl - Control de Juegos e Internet por Laboratorio" />

            {loading && <div>Cargando GameControl...</div>}

            {!loading && status && (
                <>
                    <div className="mb-3">
                        <span className="font-weight-bold mr-2">Estado del Módulo:</span>
                        <span className={`badge ${status.enabled ? 'badge-success' : 'badge-secondary'}`}>
                            {status.enabled ? 'Activo' : 'Inactivo'}
                        </span>
                    </div>

                    {error && <div className="alert alert-danger">{error}</div>}

                    <div className="mb-3">
                        <input
                            type="text"
                            className="form-control"
                            placeholder="Buscar por Nombre de Equipo (PC1, PC2...) o IP..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                        />
                    </div>

                    {status.labs.map(renderLab)}

                    {status.labs.length === 0 && (
                        <div className="alert alert-info">
                            Todavía no hay laboratorios. Agrega uno en la sección &quot;Laboratorios&quot; de abajo.
                        </div>
                    )}

                    <LabsAdmin labs={status.labs} onChanged={refresh} />
                </>
            )}
        </div>
    );
};

export default GameControl;
