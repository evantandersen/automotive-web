package main

import "github.com/charliehorse55/automotiveSim"
import "net/http"
import "io/ioutil"
import "log"
import "encoding/json"
import "os"
import "fmt"


type simulationResult struct {
    Accel100 string
    QuarterMile string
    TopSpeed string
	TopSpeedAccelTime string
	TopSpeedEff string
    CityEff string
    HighwayEff string
    PeakG string
    Cruise120 string
    
    //from the acceleration simulation
    Speed []float64
    Power []float64
    Acceleration []float64
    
    // Wh/km vs speed
    Efficiency []float64
}

var epaUDDS automotiveSim.SimulatorRun
var epaHighway automotiveSim.SimulatorRun


func handler(w http.ResponseWriter, r *http.Request) {
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Fatal(err)
    }
    
    vehicle, err  := automotiveSim.Parse(body)
    if err != nil {
        return
    }
        
    //run an acceleration simulation
    sim := automotiveSim.InitSimulation(vehicle)
    
    var result simulationResult
    
    var topSpeed float64
    var peakG float64
    
    //top speed is reached once acceleration is less than 0.01 m/s^2
    for i := 0; ; i++ {        
        //request to accelerate much faster than is possible
        sim.Tick(10000)
        
        if(sim.Acceleration > peakG) {
            peakG = sim.Acceleration
        }
        
        //have he accelerated past 100 kph?
        if sim.Speed > (100/3.6) && result.Accel100 == "" {
            result.Accel100 = fmt.Sprintf("%5.2fs", sim.Time)
        }
        
        //quarter mile in meteres
        if sim.Distance > 402.33600 && result.QuarterMile == "" {
            result.QuarterMile = fmt.Sprintf("%5.2fs", sim.Time)
        }
        
        //record data until we hit top speed
        if result.TopSpeed == ""  && (i % 100) == 0 {
            result.Speed = append(result.Speed, sim.Speed)
            result.Power = append(result.Power, sim.PowerUse)
            result.Acceleration = append(result.Acceleration, sim.Acceleration)
        }
        
        //check if we have reached top speed
        if sim.Acceleration < 0.01 && result.TopSpeed == "" {
            result.TopSpeed = fmt.Sprintf("%3.0f kph", sim.Speed*3.6)
			result.TopSpeedAccelTime = fmt.Sprintf("%5.2fs", sim.Time)
            topSpeed = sim.Speed
            //we can't accelerate to that speed if it's higher than our top speed!
            if sim.Speed < (100/3.6) {
                result.Accel100 = "Top Speed < 100 kph"
            }
        }
                
        //we are done with this simulation
        if result.TopSpeed != "" && result.Accel100 != "" && result.QuarterMile != "" {
            break
        }
    }
    
    result.PeakG = fmt.Sprintf("%4.2fg", peakG/9.81)
    
    //calculate wh/km from 1 to 200 km/hr
    effSpeed := 150
    if int(topSpeed*3.6) < effSpeed {
        effSpeed  = int(topSpeed*3.6)
    }
    
    for i := 30; i <= effSpeed; i++ {
        sim.Speed = float64(i)/3.6;
        sim.Tick(0);
        if i == 120 {
            result.Cruise120 = fmt.Sprintf("%5.2f L/100km equivalent", (sim.PowerUse/float64(i))*0.01123595505)
        }
        result.Efficiency = append(result.Efficiency, sim.PowerUse/float64(i))
    }
	sim.Speed = topSpeed
	sim.Tick(0)
    result.TopSpeedEff = fmt.Sprintf("%5.2f L/100km equivalent", (sim.PowerUse/topSpeed)*0.00312109862)
	
	
    eff, err := automotiveSim.RunInput(&epaUDDS, vehicle)
    if err != nil {
        result.CityEff = err.Error()
    } else {
        result.CityEff = fmt.Sprintf("%5.2f L/100km equivalent", eff*0.00312109862)
    }
    
    eff, err = automotiveSim.RunInput(&epaHighway, vehicle)
    if err != nil {
        result.HighwayEff = err.Error()
    } else {
        result.HighwayEff = fmt.Sprintf("%5.2f L/100km equivalent", eff*0.00312109862)
    }

    data, err := json.Marshal(result)
    if err != nil {
        log.Printf("Failed to marshal data: %v\n", result)
        return
    }
    
    w.Write(data)
}

func main() {
    b, err := ioutil.ReadFile("udds.json")
    if err != nil {
        log.Fatal(err)
    }
    err = json.Unmarshal(b, &epaUDDS)
    if err != nil {
        log.Fatal(err)
    }

    b, err = ioutil.ReadFile("highway.json")
    if err != nil {
        log.Fatal(err)
    }
    err = json.Unmarshal(b, &epaHighway)
    if err != nil {
        log.Fatal(err)
    }

    
    http.HandleFunc("/simulate", handler)
    http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("."))))
    log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
