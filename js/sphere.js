// Colour hex codes
colors = { BLACK: 0x000000, WHITE: 0xffffff };

// The main spherical properties we want to use
var numberOfPoints = 250;
var sphereRadius = 25;

var pointCoordinates = generatePointCoordinates(numberOfPoints, sphereRadius);

// The scene's local y rotation expressed in radians. This controls how quickly the
// sphere rotates.
var rotationSpeed = 0.008;

// Generate and render the scene
generateScene(pointCoordinates, rotationSpeed);

// This function generates a list of world point coordinates evenly distributed on
// the surface of our sphere and returns them.
function generatePointCoordinates(numberOfPoints, sphereRadius) {
  var points = [];

  for (var i = 0; i < numberOfPoints; i++) {
    // Calculate the appropriate z increment / unit sphere z coordinate
    // so that we distribute our points evenly between the interval [-1, 1]
    var z_increment = 1 / numberOfPoints;
    var unit_sphere_z = 2 * i * z_increment - 1 + z_increment;

    // Calculate the unit sphere cross sectional radius cutting through the
    // x-y plane at point z
    var x_y_radius = Math.sqrt(1 - Math.pow(unit_sphere_z, 2));

    // Calculate the azimuthal angle (phi) so we can try to evenly distribute
    // our points on our spherical surface
    var phi_angle_increment = 2.4; // approximation of Math.PI * (3 - Math.sqrt(5));
    var phi = (i + 1) * phi_angle_increment;

    var unit_sphere_x = Math.cos(phi) * x_y_radius;
    var unit_sphere_y = Math.sin(phi) * x_y_radius;

    // Calculate the (x, y, z) world point coordinates
    x = unit_sphere_x * sphereRadius;
    y = unit_sphere_y * sphereRadius;
    z = unit_sphere_z * sphereRadius;

    var point = {
      x: x,
      y: y,
      z: z
    };

    points.push(point);
  }

  return points;
}

function generateScene(pointCoordinates, rotationSpeed) {
  var scene = new THREE.Scene();

  scene.background = new THREE.Color(colors.WHITE);

  // Frustum variables to use for the perspective camera
  var fieldOfView = 45;
  var aspect = window.innerWidth / window.innerHeight;
  var nearPlane = 1;
  var farPlane = 600;

  camera = new THREE.PerspectiveCamera(
    fieldOfView,
    aspect,
    nearPlane,
    farPlane
  );

  // Set the camera position to (x = 0, y = 0, z = 80) in world space.
  camera.position.x = 0;
  camera.position.y = 0;
  camera.position.z = 125;

  // Rotate the camera to face the point (x = 0, y = 0, z = 0) in world space.
  camera.lookAt(new THREE.Vector3(0, 0, 0));

  var renderer = new THREE.WebGLRenderer();
  renderer.setSize(window.innerWidth, window.innerHeight);

  // Add the renderer canvas (where the renderer draws its output) to the page.
  document.getElementById('sphere-container').appendChild(renderer.domElement);

  for (var i = 0; i < pointCoordinates.length; i++) {
    // Create the spherical point
    var pointRadius = 0.25;
    var geometry = new THREE.SphereGeometry(pointRadius);
    var material = new THREE.MeshBasicMaterial({ color: colors.BLACK });
    var point = new THREE.Mesh(geometry, material);

    // Set the point coordinates and add the point to our scene

    var pointCoordinate = pointCoordinates[i];

    point.position.x = pointCoordinate.x;
    point.position.y = pointCoordinate.y;
    point.position.z = pointCoordinate.z;

    scene.add(point);
    
  }

  function render() {
    // Set the scene y rotation to the appropriate speed and render the scene
    scene.rotation.y += rotationSpeed;
    requestAnimationFrame(render);
    renderer.render(scene, camera);
  }

  render();
}
