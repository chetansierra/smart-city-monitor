import { MAPS_SCRIPT_ID } from '../constants/app';

declare global {
  interface Window {
    google?: any;
  }
}

let googleMapsLoadPromise: Promise<void> | null = null;

export const loadGoogleMaps = (apiKey: string): Promise<void> => {
  if (!apiKey) {
    return Promise.reject(new Error('Missing Google Maps API key'));
  }
  if (window.google?.maps) {
    return Promise.resolve();
  }
  if (googleMapsLoadPromise) {
    return googleMapsLoadPromise;
  }
  googleMapsLoadPromise = new Promise((resolve, reject) => {
    const isReady = () => Boolean(window.google?.maps);

    const resolveReady = () => {
      if (isReady()) {
        resolve();
        return true;
      }
      return false;
    };

    const existing = document.getElementById(MAPS_SCRIPT_ID) as HTMLScriptElement | null;
    if (existing) {
      if (resolveReady()) {
        existing.dataset.loaded = 'true';
        return;
      }

      let pollId: number | null = null;
      let timeoutId: number | null = null;

      const cleanup = () => {
        existing.removeEventListener('load', handleLoad);
        existing.removeEventListener('error', handleError);
        if (pollId !== null) {
          window.clearInterval(pollId);
        }
        if (timeoutId !== null) {
          window.clearTimeout(timeoutId);
        }
      };

      const handleLoad = () => {
        existing.dataset.loaded = 'true';
        if (isReady()) { cleanup(); resolve(); return; }
        // google.maps may not be ready yet — poll until it is
        let elapsed = 0;
        const poll = window.setInterval(() => {
          elapsed += 50;
          if (isReady()) {
            window.clearInterval(poll);
            cleanup();
            resolve();
          } else if (elapsed >= 10_000) {
            window.clearInterval(poll);
            cleanup();
            reject(new Error('Google Maps API did not initialize after script load'));
          }
        }, 50);
      };
      const handleError = () => {
        cleanup();
        reject(new Error('Failed to load Google Maps'));
      };

      existing.addEventListener('load', handleLoad, { once: true });
      existing.addEventListener('error', handleError, { once: true });

      pollId = window.setInterval(() => {
        if (!resolveReady()) return;
        existing.dataset.loaded = 'true';
        cleanup();
      }, 60);

      timeoutId = window.setTimeout(() => {
        cleanup();
        reject(new Error('Timed out while loading Google Maps'));
      }, 10000);
      return;
    }

    const script = document.createElement('script');
    script.id = MAPS_SCRIPT_ID;
    script.src = `https://maps.googleapis.com/maps/api/js?key=${encodeURIComponent(apiKey)}&loading=async&libraries=marker`;
    script.async = true;
    script.defer = true;
    script.onload = () => {
      script.dataset.loaded = 'true';
      if (isReady()) { resolve(); return; }
      // google.maps may not be ready yet — poll until it is
      let elapsed = 0;
      const poll = window.setInterval(() => {
        elapsed += 50;
        if (isReady()) {
          window.clearInterval(poll);
          resolve();
        } else if (elapsed >= 10_000) {
          window.clearInterval(poll);
          reject(new Error('Google Maps API did not initialize after script load'));
        }
      }, 50);
    };
    script.onerror = () => reject(new Error('Failed to load Google Maps'));
    document.head.appendChild(script);
  });

  googleMapsLoadPromise = googleMapsLoadPromise.catch((err) => {
    googleMapsLoadPromise = null;
    throw err;
  });

  return googleMapsLoadPromise;
};
