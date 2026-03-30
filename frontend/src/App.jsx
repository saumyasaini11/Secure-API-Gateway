import React, { useState, useEffect, useRef } from 'react';
import Globe from 'react-globe.gl';
import { Shield, Activity, Lock, Terminal as TerminalIcon, AlertTriangle, ShieldCheck, AlertOctagon, Zap } from 'lucide-react';

export default function App() {
  const [logs, setLogs] = useState([]);
  const [requests, setRequests] = useState([
    { client: 'factory-C', route: '/api/control/actuator', status: 502, latency: '3ms', time: '6:17:27 PM' },
    { client: 'factory-B', route: '/api/scada/status', status: 502, latency: '4ms', time: '6:17:12 PM' },
    { client: 'factory-A', route: '/api/scada/sensor-data', status: 502, latency: '3ms', time: '6:17:04 PM' },
    { client: 'scada-node-01', route: '/api/scada/sensor-data', status: 200, latency: '24ms', time: '6:01:14 PM' },
    { client: 'scada-node-01', route: '/api/scada/status', status: 200, latency: '5ms', time: '4:25:11 PM' },
  ]);
  const [dimensions, setDimensions] = useState({ width: window.innerWidth, height: window.innerHeight });

  const globeRef = useRef();

  // Generate Internet Arcs
  const [arcsData] = useState(() => {
    return [...Array(30).keys()].map(() => ({
      startLat: (Math.random() - 0.5) * 160,
      startLng: (Math.random() - 0.5) * 360,
      endLat: (Math.random() - 0.5) * 160,
      endLng: (Math.random() - 0.5) * 360,
      color: ['rgba(255, 255, 255, 0.4)', 'rgba(56, 189, 248, 0.4)'][Math.floor(Math.random() * 2)]
    }));
  });

  useEffect(() => {
    const handleResize = () => setDimensions({ width: window.innerWidth, height: window.innerHeight });
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  useEffect(() => {
    if (globeRef.current) {
      globeRef.current.controls().autoRotate = true;
      globeRef.current.controls().autoRotateSpeed = 0.5;
      globeRef.current.controls().enableZoom = false;
      globeRef.current.pointOfView({ altitude: 2 });
    }
  }, []);

  // Simulated Terminal Logs
  useEffect(() => {
    const interval = setInterval(() => {
      setLogs(prev => {
        const IPs = ['192.168.1.45', '10.0.0.99', '172.16.254.1', '14.22.90.100'];
        const isSuspicious = Math.random() > 0.8;
        const newLog = `> [SYS] INTERCEPT: ${IPs[Math.floor(Math.random() * IPs.length)]} - ${isSuspicious ? 'WARN' : 'OK'}`;
        const nextLogs = [...prev, newLog];
        return nextLogs.slice(-6); // KEEP LAST 6
      });
    }, 1500);
    return () => clearInterval(interval);
  }, []);

  // Real-time requests
  useEffect(() => {
    const interval = setInterval(() => {
      const clients = ['factory-A', 'factory-B', 'factory-C', 'scada-node-01', 'scada-node-02'];
      const routes = ['/api/scada/sensor-data', '/api/control/actuator', '/api/scada/status', '/api/auth/token'];
      const newReq = {
        client: clients[Math.floor(Math.random() * clients.length)],
        route: routes[Math.floor(Math.random() * routes.length)],
        status: Math.random() > 0.85 ? 502 : 200,
        latency: Math.floor(Math.random() * 40 + 2) + 'ms',
        time: new Date().toLocaleTimeString('en-US')
      };
      setRequests(prev => [newReq, ...prev].slice(0, 5));
    }, 3500);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="relative min-h-screen font-sans bg-[#050505] text-gray-200 overflow-x-hidden selection:bg-primary/30">
      
      {/* Globe Background */}
      <div className="fixed inset-0 z-0 flex items-center justify-center opacity-60 pointer-events-none -mr-[20vw]">
        <Globe
          ref={globeRef}
          width={dimensions.width}
          height={dimensions.height}
          backgroundColor="rgba(0,0,0,0)"
          globeImageUrl="//unpkg.com/three-globe/example/img/earth-dark.jpg"
          atmosphereColor="#ffffff"
          atmosphereAltitude={0.15}
          arcsData={arcsData}
          arcColor={'color'}
          arcDashLength={() => Math.random()}
          arcDashGap={() => Math.random()}
          arcDashAnimateTime={() => Math.random() * 4000 + 2000}
        />
      </div>

      <div className="relative z-10 w-full max-w-7xl mx-auto px-4 md:px-8 py-10 flex flex-col pt-16">
        
        {/* Navigation Navbar */}
        <nav className="glass-panel px-8 py-4 flex justify-between items-center mb-16 w-full sticky top-4 z-50">
          <div className="flex items-center gap-3 text-white font-bold tracking-[0.2em] text-sm uppercase">
            <ShieldCheck size={20} className="text-secondary" />
            <span>Voicura<span className="text-tertiary ml-2 font-normal">Command</span></span>
          </div>
          <div className="hidden md:flex gap-10 text-xs text-tertiary uppercase tracking-widest">
            <a href="#dashboard" className="hover:text-primary transition-colors">How It Works</a>
            <a href="#about" className="hover:text-primary transition-colors">About</a>
            <a href="#pricing" className="hover:text-primary transition-colors">Pricing</a>
            <a href="#community" className="text-secondary hover:text-white transition-colors">Community</a>
          </div>
        </nav>

        {/* SECTION 1: GLOBAL THREAT INTELLIGENCE */}
        <section id="dashboard" className="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center mb-32">
          <div className="flex flex-col space-y-8 max-w-lg">
            <div className="inline-block border border-tertiary/20 px-3 py-1 text-tertiary text-[10px] uppercase tracking-[0.2em] glass-panel w-max">
              Minimalist Network Defense
            </div>
            <h1 className="text-5xl md:text-6xl font-light tracking-tight text-white leading-[1.1]">
              YOUR SILENCE.<br/>
              <span className="font-bold">SECURED.</span>
            </h1>
            <p className="text-gray-400 text-sm max-w-sm leading-relaxed tracking-wide">
              A privacy-first cybersecurity service that encrypts your presence, protects your voice, and vanishes your digital footprint—elegantly.
            </p>
            <div className="flex gap-4 pt-4">
              <button className="bg-white text-black px-6 py-2.5 text-xs font-bold uppercase tracking-widest hover:bg-white/90 transition-all duration-300">
                Get Started
              </button>
              <button className="glass-panel text-white border-transparent px-6 py-2.5 text-xs uppercase tracking-widest transition-all hover:border-white/20">
                Learn How it works
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 relative">
            <div className="glass-panel p-5 sm:col-span-2 group pt-6">
              <div className="flex justify-between items-center mb-6">
                <div className="flex items-center gap-2 text-white text-[10px] uppercase tracking-[0.2em]">
                  <AlertTriangle size={14} className="text-secondary" /> Active Threats
                </div>
                <div className="text-[10px] text-tertiary animate-pulse font-mono tracking-widest">LIVE</div>
              </div>
              <div className="space-y-3">
                {['DDoS attempt detected', 'SQL injection mitigated', 'Unauthorized access blocked'].map((threat, i) => (
                  <div key={i} className="flex justify-between items-center text-xs border-b border-white/[0.03] pb-2">
                    <span className="text-gray-400 tracking-wide">{threat}</span>
                    <span className="text-primary/80 font-mono text-[10px]">{(Math.random() * 10).toFixed(2)} ms</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="glass-panel p-5 group">
              <div className="flex items-center gap-2 text-white text-[10px] uppercase tracking-[0.2em] mb-4">
                <TerminalIcon size={14} /> Packet Analysis
              </div>
              <div className="font-mono text-[10px] text-tertiary overflow-hidden h-[120px] flex flex-col justify-end space-y-1">
                {logs.map((log, index) => (
                  <div key={index} className={log.includes('WARN') ? 'text-secondary' : 'text-gray-500'}>
                    {log}
                  </div>
                ))}
                <div className="animate-pulse">&gt; _</div>
              </div>
            </div>

            <div className="glass-panel p-5 group flex flex-col items-center justify-center min-h-[160px]">
              <Lock size={32} strokeWidth={1.5} className="text-white mb-4" />
              <div className="text-center w-full">
                <div className="text-white text-[10px] tracking-widest uppercase mb-2">AES-4096</div>
                <div className="w-full bg-white/[0.05] h-0.5 mt-2 overflow-hidden">
                  <div className="bg-primary h-full w-3/4"></div>
                </div>
              </div>
            </div>
          </div>
        </section>


        {/* SECTION 2: API GATEWAY */}
        <section id="monitor" className="w-full space-y-6">
          <div className="flex items-center gap-4 mb-8">
            <div className="p-2 glass-panel">
              <Shield size={18} className="text-white" />
            </div>
            <div>
              <h2 className="text-lg font-light text-white uppercase tracking-[0.2em]">API Gateway</h2>
              <div className="text-[10px] text-gray-500 uppercase tracking-[0.3em] mt-1">Industrial Security Monitor</div>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="glass-panel p-6">
              <div className="flex justify-between items-center mb-6 border-b border-white/[0.05] pb-4">
                <h3 className="text-white text-[10px] uppercase tracking-[0.2em]">Recent Requests</h3>
              </div>
              
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="text-[9px] text-tertiary uppercase tracking-widest font-mono border-b border-white/[0.05]">
                      <th className="pb-3 pt-1 font-normal">Client</th>
                      <th className="pb-3 pt-1 font-normal">Route</th>
                      <th className="pb-3 pt-1 font-normal text-right">Status</th>
                    </tr>
                  </thead>
                  <tbody className="text-xs font-mono align-middle">
                    {requests.map((req, i) => (
                      <tr key={i} className="hover:bg-white/[0.01] transition-colors group">
                        <td className="py-2.5 text-gray-400 border-b border-white/[0.02]">{req.client}</td>
                        <td className="py-2.5 text-gray-500 border-b border-white/[0.02]">{req.route}</td>
                        <td className="py-2.5 text-right border-b border-white/[0.02]">
                          <span className={`${req.status === 200 ? 'text-primary' : 'text-secondary'}`}>
                            {req.status}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="glass-panel p-6">
              <div className="flex justify-between items-center mb-6 border-b border-white/[0.05] pb-4">
                <h3 className="text-white text-[10px] uppercase tracking-[0.2em]">Client Usage</h3>
                <span className="text-[10px] text-tertiary font-mono">24H</span>
              </div>

              <div className="space-y-5">
                {[
                  { name: 'scada-node-01', val: 78, num: 2 },
                  { name: 'factory-A', val: 45, num: 1 },
                  { name: 'factory-B', val: 60, num: 1 },
                  { name: 'factory-C', val: 30, num: 1 },
                ].map((client, i) => (
                  <div key={i} className="flex items-center gap-4 text-xs font-mono">
                    <div className="w-28 text-gray-400">{client.name}</div>
                    <div className="flex-1 h-[2px] bg-white/[0.05] relative">
                      <div className="absolute top-0 left-0 bottom-0 bg-primary" style={{ width: `${client.val}%` }}></div>
                    </div>
                    <div className="w-4 text-right text-gray-500">{client.num}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="glass-panel p-6 flex flex-col justify-between">
              <div className="flex justify-between items-center mb-6">
                <h3 className="text-white text-[10px] uppercase tracking-[0.2em] flex items-center gap-2">
                  <AlertOctagon size={12} className="text-secondary" /> Brute Force
                </h3>
              </div>
              <div className="flex-1 flex flex-col justify-center items-center text-gray-400 text-sm py-8">
                <div className="flex items-center gap-2 mb-2">
                  <ShieldCheck size={16} className="text-primary" />
                  <span className="tracking-wide">No threats detected</span>
                </div>
              </div>
            </div>

            <div className="glass-panel p-6">
              <div className="flex justify-between items-center mb-6 border-b border-white/[0.05] pb-4">
                <h3 className="text-white text-[10px] uppercase tracking-[0.2em] flex items-center gap-2">
                  <Zap size={12} /> JWT Generator
                </h3>
              </div>

              <div className="flex flex-col gap-3">
                <input 
                  type="text" 
                  placeholder="client_id" 
                  className="w-full bg-white/[0.02] border border-white/[0.05] py-2.5 px-3 text-xs text-white font-mono focus:outline-none focus:border-primary/50 transition-all"
                />
                <button className="bg-white text-black hover:bg-white/90 transition-colors font-bold uppercase tracking-[0.2em] py-2.5 text-[10px]">
                  Generate
                </button>
              </div>
            </div>
          </div>
        </section>

        {/* SECTION 3: ABOUT THE COMPANY */}
        <section id="about" className="w-full space-y-12 mt-24 border-t border-white/[0.05] pt-24 pb-32">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-16">
            <div className="space-y-6">
              <div className="flex items-center gap-4">
                <div className="w-8 h-[1px] bg-primary"></div>
                <span className="text-[10px] text-gray-500 uppercase tracking-[0.3em]">Corporate Intelligence</span>
              </div>
              <h2 className="text-4xl md:text-5xl font-light text-white tracking-tight leading-tight">
                A CALM AND DELIBERATE PRESENCE<br />
                <span className="text-gray-500">IN A DIGITAL WORLD</span>
              </h2>
              <p className="text-gray-400 text-sm leading-relaxed max-w-md pt-4">
                We are dedicated to pioneering invisible security meshes across the globe. We operate silently to ensure your industrial networks and digital operations remain resilient, untraceable, and continuously fortified against emerging cyber-threat vectors without asking for attention.
              </p>
              
              <div className="flex flex-col sm:flex-row gap-8 pt-8 border-t border-white/[0.02]">
                <div>
                  <div className="text-3xl font-light text-white mb-1">58K+</div>
                  <div className="text-[10px] uppercase tracking-widest text-primary">Global Nodes</div>
                </div>
                <div>
                  <div className="text-3xl font-light text-white mb-1">74.5M+</div>
                  <div className="text-[10px] uppercase tracking-widest text-white/50">Threats Neutralized</div>
                </div>
                <div>
                  <div className="text-3xl font-light text-white mb-1">99.9%</div>
                  <div className="text-[10px] uppercase tracking-widest text-white/50">Uptime Protocol</div>
                </div>
              </div>
            </div>

            <div className="relative glass-panel p-8 sm:p-12 flex flex-col justify-center min-h-[300px] group overflow-hidden">
              <div className="absolute inset-0 bg-primary/5 opacity-0 group-hover:opacity-100 transition-opacity duration-700"></div>
              <div className="relative z-10 space-y-8 flex flex-col items-start border-l border-white/10 pl-6 sm:pl-8">
                 <h3 className="text-white text-xs tracking-[0.2em] uppercase">Three-Layer Silent Defense</h3>
                 <ul className="text-gray-500 text-xs space-y-6 font-mono w-full">
                   <li className="flex items-center gap-4">
                     <span className="text-primary border border-primary/20 px-2 py-1 bg-primary/5 rounded">01</span>
                     <span>Neural Edge Quantum Encryption</span>
                   </li>
                   <li className="flex items-center gap-4">
                     <span className="text-primary border border-primary/20 px-2 py-1 bg-primary/5 rounded">02</span>
                     <span>Sovereign Identity Architecture</span>
                   </li>
                   <li className="flex items-center gap-4">
                     <span className="text-primary border border-primary/20 px-2 py-1 bg-primary/5 rounded">03</span>
                     <span>Autonomous Threat Suppression</span>
                   </li>
                 </ul>
              </div>
            </div>
          </div>
        </section>

      </div>
    </div>
  );
}
