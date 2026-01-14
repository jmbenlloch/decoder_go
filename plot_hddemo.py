import sys
import argparse
import itertools

import numpy as np
import tables as tb

from matplotlib import pylab as plt

_colors = itertools.cycle(["k", "c", "r", "g", "y", "b", "dimgrey", "chocolate", "m", "gold", "tomato", "lime"])

def mode(data):
    unique_values, occurrences = np.unique(data, return_counts=True)
    return unique_values[np.argmax(occurrences)]


def customize_plot(zoomx, zoomy, wf_type, evt, elecid=None):
    title  = ("{} | Evt {}"           .format(wf_type, evt        ) if elecid is None else
              "{} | Evt {}, elecid {}".format(wf_type, evt, elecid))


    plt.xlabel("Time bin")
    plt.ylabel("Amplitude (adc)")
    plt.title(title)

    if zoomx: plt.xlim(zoomx)
    if zoomy: plt.ylim(zoomy)
    if elecid is None: plt.legend()


def show_and_wait():
    plt.show()
    input("Press [enter] to continue")
    plt.clf()


def plot_waveforms(waveforms, sensors, baselines, evt, *, wf_type="PMT", range=(None,),
                   overlay=False, sum=False,
                   zoomx=False, zoomy=False, dual=False,
                   trg=56000):
    range = slice(*range)

    wfsize = waveforms.shape[2]
    time = np.arange(wfsize).astype(float)
    if   wf_type == "PMT" : time /= 40
    elif wf_type == "BLR" : time /= 40
    elif wf_type == "SiPM": pass
    else: raise ValueError("Unrecognized wf type {}. ".format(wf_type) +
                           "Valid options: are 'PMT', 'BLR' and 'SiPM'")

    if sum: wf_type += " SUM"

    gmin, gmax = float("inf"), -float("inf")
    plt.ion()
    ax1 = plt.gca()
    if sum:
        sum_wf = np.zeros(waveforms.shape[2])

    if dual:
        for wf, wf_dual, ID, color in zip(waveforms[0][range], waveforms[1][range] , sensors[range], _colors):
            ymin, ymax = min(wf_dual), max(wf_dual)
            if ymin < gmin: gmin = ymin
            if ymax > gmax: gmax = ymax

            plt.plot(wf, drawstyle="steps", label=str(ID[0]), c=color)
            plt.plot(wf_dual, drawstyle="steps", label=str(ID[0]), c=next(_colors))

            ylim = (0.99 * ymin, 1.01 * ymax)
            customize_plot(zoomx, zoomy if zoomy else ylim, wf_type, evt, ID[0])
            show_and_wait()
    else:
        for i, (wf, ID, color) in enumerate(zip(waveforms[0][range], sensors[range], _colors)):
            ymin, ymax = min(wf), max(wf)
            if ymin < gmin: gmin = ymin
            if ymax > gmax: gmax = ymax

            if sum:
                bls_wf = wf - mode(wf)
                sum_wf = sum_wf + bls_wf * (1 if "SiPM" in wf_type else -1)
            else:
                plt.plot(wf, drawstyle="steps", label=str(ID[0]), c=color)
                #if len(baselines) > 0:
                #    plt.axhline(y=baselines[0][i], color='r', linestyle='-')
                ylim = (0.99 * ymin, 1.01 * ymax)
                customize_plot(zoomx, zoomy if zoomy else ylim, wf_type, evt, ID[0])

            if trg > 0:                                     
                plt.vlines(x=trg, ymin=0, ymax=4096)                                      
            if not overlay and not sum:
                if len(baselines) > 0:
                    if ymin >= baselines[0][i]:
                        ymin = 0.99 * baselines[0][i]
                    if ymax <= baselines[0][i]:
                        ymax = 1.01 * baselines[0][i]
                ylim = (0.99 * ymin, 1.01 * ymax)

                customize_plot(zoomx, zoomy if zoomy else ylim, wf_type, evt, ID[0])
                show_and_wait()

    if overlay:
        ylim = 0.99 * gmin, 1.01 * gmax
        customize_plot(zoomx, zoomy if zoomy else ylim, wf_type, evt)
        show_and_wait()

    if sum:
        ylim = np.min(sum_wf) - 50, np.max(sum_wf) + 50
        plt.plot(sum_wf, drawstyle="steps", c="k")
        customize_plot(zoomx, zoomy if zoomy else ylim, wf_type, evt)
        show_and_wait()


def plot_file(filename, rwf=True, blr=True, sipm=True, sipm_range=(None,),
              overlay=False, sum=False, first=0, pmt_sum=True,
              zoomx=False, zoomy=False, dual=False, elecid=False, trg=56000):
    with tb.open_file(filename) as file:
        evt_step = 2 if dual else 1
        event_numbers = file.root.Run.events[:]
        if len(sipm_range) > 1:
            sipm_channels = file.root.Sensors.DataSiPM.cols.sensorID[:]
            if elecid:
                sipm_channels = file.root.Sensors.DataSiPM.cols.channel[:]
            start_idx = np.where(sipm_channels == sipm_range[0])[0][0]
            end_idx   = np.where(sipm_channels == sipm_range[1])[0][0]
            sipm_range = (start_idx, end_idx)

        for evt in range(first, len(file.root.Run.events.cols.evt_number), evt_step):
            evt_number = event_numbers[evt][0]

            pmt_baselines = []
            if "RD/pmt_baselines"  in file.root:
                pmt_baselines = file.root.RD.pmt_baselines[evt:evt+1]

            if rwf  and "RD/pmtrwf"  in file.root and "Sensors/DataPMT"  in file.root:
                plot_waveforms(file.root.RD     . pmtrwf [evt : evt+evt_step],
                               file.root.Sensors.DataPMT [:], pmt_baselines,
                               evt_number, wf_type="PMT", overlay=overlay, sum=sum,
                               zoomx=zoomx, zoomy=zoomy, dual=dual, trg=trg)

            blr_baselines = []
            if "RD/pmt_baselines"  in file.root:
                blr_baselines = file.root.RD.pmt_baselines[evt:evt+1]

            if blr  and "RD/pmtblr"  in file.root and "Sensors/DataBLR"  in file.root:
                plot_waveforms(file.root.RD     . pmtblr [evt : evt+evt_step],
                               file.root.Sensors.DataBLR [:], blr_baselines,
                               evt_number, wf_type="BLR", overlay=overlay, sum=sum,
                               zoomx=zoomx, zoomy=zoomy, dual=dual, trg=trg)

            if pmt_sum  and "RD/pmt_sum"  in file.root:
                wf_sum = file.root.RD.pmt_sum[evt:evt+evt_step]
                wf_sum = wf_sum.reshape(1,1,wf_sum.shape[1])
                pmt_sum_baseline = file.root.RD.pmt_sum_baseline[evt:evt+1]
                plot_waveforms(wf_sum,
                               file.root.Sensors.DataPMT [:], pmt_sum_baseline, #[],
                               evt_number, wf_type="PMT", overlay=overlay, sum=sum,
                               zoomx=zoomx, zoomy=zoomy, dual=dual, trg=trg)

            sipm_baselines = []
            if sipm and "RD/sipmrwf" in file.root and "Sensors/DataSiPM" in file.root:
                plot_waveforms(file.root.RD     .sipmrwf [evt : evt + evt_step],
                               file.root.Sensors.DataSiPM[:], sipm_baselines,
                               evt_number, wf_type="SiPM", range=sipm_range,
                               overlay=overlay, sum=sum,
                               zoomx=zoomx, zoomy=zoomy, dual=dual, trg=trg)


if __name__ == '__main__':
    def sipm_index(sensor_id):
        sensor_id = int(sensor_id)
        dice      = sensor_id // 1000
        sipm_no   = sensor_id  % 1000
        return (dice - 1) * 64 + sipm_no

    parser = argparse.ArgumentParser()
    parser.add_argument("--file"      , required=True)

    parser.add_argument( "-pmt"       , action="store_true")
    parser.add_argument( "-pmt-sum"   , action="store_true")
    parser.add_argument( "-blr"       , action="store_true")
    parser.add_argument( "-sipm"      , action="store_true")
    #parser.add_argument("--sipm-range", type=sipm_index, default=(None,), nargs="*")
    parser.add_argument("--sipm-range", type=int, default=(None,), nargs="*")

    parser.add_argument("--overlay"   , action="store_true")
    parser.add_argument("--sum"       , action="store_true")
    parser.add_argument("--dual"      , action="store_true")
    parser.add_argument("--first"     , type=int, default=0)
    parser.add_argument("--zoomx"     , type=int, default=(), nargs="*")
    parser.add_argument("--zoomy"     , type=int, default=(), nargs="*")
    parser.add_argument("--elecid"    , action="store_true")
    parser.add_argument("--trg"       , type=int, default=0)

    args = parser.parse_args(sys.argv[1:])
    filename = args.file

    plot_file(filename,
              rwf=args.pmt, blr=args.blr, sipm=args.sipm, sipm_range=args.sipm_range,
              overlay=args.overlay, sum=args.sum, first=args.first, pmt_sum=args.pmt_sum,
              zoomx=args.zoomx, zoomy=args.zoomy, dual=args.dual, elecid=args.elecid,
              trg=args.trg)

